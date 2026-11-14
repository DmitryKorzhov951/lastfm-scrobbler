package lyrics

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const OVHEndpoint = "https://api.lyrics.ovh"

type OVH struct {
	HTTP      *http.Client
	UserAgent string
	Endpoint  string
}

func (c *OVH) Find(ctx context.Context, q Query) (Lyrics, error) {
	artist, title := strings.TrimSpace(q.Artist), CleanTitle(strings.TrimSpace(q.Title))
	if artist == "" || title == "" {
		return Lyrics{}, ErrNotFound
	}
	base := c.Endpoint
	if base == "" {
		base = OVHEndpoint
	}
	var out struct {
		Lyrics string `json:"lyrics"`
	}
	addr := strings.TrimRight(base, "/") + "/v1/" + url.PathEscape(artist) + "/" + url.PathEscape(title)
	found, err := fetchJSON(ctx, c.HTTP, c.UserAgent, "lyrics.ovh", addr, &out)
	if err != nil {
		return Lyrics{}, err
	}
	text := cleanOVH(out.Lyrics)
	if !found || text == "" {
		return Lyrics{}, ErrNotFound
	}
	return FromText(text, FromOVH), nil
}

var parolesRe = regexp.MustCompile(`(?i)^paroles de la chanson .+ par .+$`)

func cleanOVH(text string) string {
	lines := strings.Split(strings.TrimSpace(newlines(text)), "\n")
	if parolesRe.MatchString(strings.TrimSpace(lines[0])) {
		lines = lines[1:]
	}
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	spaced := doubleSpaced(lines)
	var out []string
	blank := 0
	for _, l := range lines {
		if l == "" {
			blank++
			continue
		}
		if len(out) > 0 && blank > 0 && (!spaced || blank > 1) {
			out = append(out, "")
		}
		blank = 0
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func doubleSpaced(lines []string) bool {
	text := 0
	for i, l := range lines {
		if l == "" {
			continue
		}
		text++
		if i+1 < len(lines) && lines[i+1] != "" {
			return false
		}
	}
	return text >= 4
}
