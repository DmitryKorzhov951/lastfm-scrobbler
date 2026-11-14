package lyrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	DefaultEndpoint = "https://lrclib.net"
	maxBody         = 4 << 20
	durationSlack   = 10.0
)

var ErrNotFound = errors.New("текст песни не найден")

type Query struct {
	Artist   string
	Title    string
	Album    string
	Duration float64
}

type Client struct {
	HTTP      *http.Client
	UserAgent string
	Endpoint  string
}

type record struct {
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

func (r record) lyrics() Lyrics {
	l := Lyrics{Source: FromLRCLIB, Synced: ParseLRC(r.SyncedLyrics)}
	if len(l.Synced) == 0 {
		l.Synced = nil
	}
	l.Plain = strings.TrimSpace(newlines(r.PlainLyrics))
	if l.Plain == "" && len(l.Synced) > 0 {
		l.Plain = plainOf(l.Synced)
	}
	if l.Plain == "" && len(l.Synced) == 0 {
		l.Instrumental = r.Instrumental
	}
	return l
}

func (c *Client) Find(ctx context.Context, q Query) (Lyrics, error) {
	q.Artist, q.Title, q.Album = strings.TrimSpace(q.Artist), strings.TrimSpace(q.Title), strings.TrimSpace(q.Album)
	if q.Artist == "" || q.Title == "" {
		return Lyrics{}, ErrNotFound
	}
	if q.Duration > 0 && q.Album != "" {
		var rec record
		params := url.Values{
			"artist_name": {q.Artist}, "track_name": {q.Title}, "album_name": {q.Album},
			"duration": {strconv.Itoa(int(math.Round(q.Duration)))},
		}
		found, err := c.get(ctx, "/api/get", params, &rec)
		if err != nil {
			return Lyrics{}, err
		}
		if l := rec.lyrics(); found && !l.Empty() {
			return l, nil
		}
	}
	titles := []string{q.Title}
	if t := CleanTitle(q.Title); t != "" && t != q.Title {
		titles = append(titles, t)
	}
	for _, title := range titles {
		var recs []record
		if _, err := c.get(ctx, "/api/search", url.Values{"artist_name": {q.Artist}, "track_name": {title}}, &recs); err != nil {
			return Lyrics{}, err
		}
		if rec, ok := pick(recs, q); ok {
			return rec.lyrics(), nil
		}
	}
	var recs []record
	if _, err := c.get(ctx, "/api/search", url.Values{"q": {q.Artist + " " + titles[len(titles)-1]}}, &recs); err != nil {
		return Lyrics{}, err
	}
	if rec, ok := pick(recs, q); ok {
		return rec.lyrics(), nil
	}
	return Lyrics{}, ErrNotFound
}

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) (bool, error) {
	base := c.Endpoint
	if base == "" {
		base = DefaultEndpoint
	}
	return fetchJSON(ctx, c.HTTP, c.UserAgent, "lrclib", strings.TrimRight(base, "/")+path+"?"+params.Encode(), out)
}

func fetchJSON(ctx context.Context, hc *http.Client, agent, site, addr string, out any) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", agent)
	req.Header.Set("Accept", "application/json")
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return false, fmt.Errorf("%s: %w", site, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return false, fmt.Errorf("%s: чтение ответа: %w", site, err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("%s: ответ %d", site, resp.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return false, fmt.Errorf("%s: разбор ответа: %w", site, err)
	}
	return true, nil
}

func pick(recs []record, q Query) (record, bool) {
	artist := squash(q.Artist)
	title := squash(CleanTitle(q.Title))
	full := squash(q.Title)
	album := squash(q.Album)
	best, score := record{}, -1
	for _, r := range recs {
		if r.lyrics().Empty() {
			continue
		}
		if a := squash(r.ArtistName); a == "" || !(strings.Contains(a, artist) || strings.Contains(artist, a)) {
			continue
		}
		if t := squash(r.TrackName); t != full && squash(CleanTitle(r.TrackName)) != title {
			continue
		}
		s := 0
		if q.Duration > 0 && r.Duration > 0 {
			d := math.Abs(q.Duration - r.Duration)
			if d > durationSlack {
				continue
			}
			switch {
			case d <= 2:
				s += 4
			case d <= 5:
				s += 2
			}
		}
		if r.SyncedLyrics != "" {
			s += 3
		}
		if album != "" && squash(r.AlbumName) == album {
			s++
		}
		if s > score {
			best, score = r, s
		}
	}
	return best, score >= 0
}

var (
	featRe     = regexp.MustCompile(`(?i)\s*[\(\[](?:feat\.?|ft\.?|featuring|with)\s[^\)\]]*[\)\]]`)
	featTailRe = regexp.MustCompile(`(?i)\s+(?:feat\.?|ft\.?|featuring)\s.*$`)
	remasterRe = regexp.MustCompile(`(?i)\s*(?:-\s*(?:\d{4}\s+)?(?:digital\s+)?remaster(?:ed)?(?:\s+\d{4})?(?:\s+version)?|[\(\[](?:\d{4}\s+)?(?:digital\s+)?remaster(?:ed)?(?:\s+\d{4})?(?:\s+version)?[\)\]])\s*$`)
)

func CleanTitle(s string) string {
	s = featRe.ReplaceAllString(s, "")
	s = featTailRe.ReplaceAllString(s, "")
	s = remasterRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func squash(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == 'ё':
			b.WriteRune('е')
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}
