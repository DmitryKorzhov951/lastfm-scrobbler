package library

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const TopSize = 10

type Period struct {
	Kind    string     `json:"kind"`
	Offset  int        `json:"offset"`
	From    time.Time  `json:"from"`
	To      time.Time  `json:"to"`
	Label   string     `json:"label"`
	Live    bool       `json:"live"`
	Since   *time.Time `json:"since,omitempty"`
	HasPrev bool       `json:"hasPrev"`
	HasNext bool       `json:"hasNext"`
}

type TopItem struct {
	Rank   int    `json:"rank"`
	Artist string `json:"artist"`
	Title  string `json:"title,omitempty"`
	Album  string `json:"album,omitempty"`
	Count  int    `json:"count"`
}

type Top struct {
	Period Period    `json:"period"`
	Kind   string    `json:"kind"`
	Items  []TopItem `json:"items"`
	Total  int       `json:"total"`
}

type CardItem struct {
	Artist string `json:"artist"`
	Title  string `json:"title,omitempty"`
	Album  string `json:"album,omitempty"`
	Count  int    `json:"count"`
}

type Card struct {
	Kind        string     `json:"kind"`
	Artist      string     `json:"artist"`
	Title       string     `json:"title,omitempty"`
	Album       string     `json:"album,omitempty"`
	Count       int        `json:"count"`
	First       *time.Time `json:"first,omitempty"`
	Last        *time.Time `json:"last,omitempty"`
	Week        int        `json:"week"`
	Month       int        `json:"month"`
	Year        int        `json:"year"`
	ArtistCount int        `json:"artistCount,omitempty"`
	AlbumCount  int        `json:"albumCount,omitempty"`
	Tracks      []CardItem `json:"tracks,omitempty"`
	Albums      []CardItem `json:"albums,omitempty"`
}

var (
	monthsNom = [...]string{"январь", "февраль", "март", "апрель", "май", "июнь", "июль",
		"август", "сентябрь", "октябрь", "ноябрь", "декабрь"}
	monthsGen = [...]string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля",
		"августа", "сентября", "октября", "ноября", "декабря"}
)

func Bounds(kind string, offset int, now time.Time) (time.Time, time.Time, error) {
	y, m, d := now.Date()
	loc := now.Location()
	switch kind {
	case "week":
		back := (int(now.Weekday()) - int(time.Thursday) + 7) % 7
		from := time.Date(y, m, d-back+7*offset, 0, 0, 0, 0, loc)
		return from, time.Date(y, m, d-back+7*offset+7, 0, 0, 0, 0, loc), nil
	case "month":
		return time.Date(y, m+time.Month(offset), 1, 0, 0, 0, 0, loc), time.Date(y, m+time.Month(offset)+1, 1, 0, 0, 0, 0, loc), nil
	case "year":
		return time.Date(y+offset, 1, 1, 0, 0, 0, 0, loc), time.Date(y+offset+1, 1, 1, 0, 0, 0, 0, loc), nil
	}
	return time.Time{}, time.Time{}, fmt.Errorf("неизвестный период %q", kind)
}

func label(kind string, from, to time.Time) string {
	switch kind {
	case "week":
		last := to.AddDate(0, 0, -1)
		switch {
		case from.Year() != last.Year():
			return fmt.Sprintf("%d %s %d – %d %s %d", from.Day(), monthsGen[from.Month()-1], from.Year(),
				last.Day(), monthsGen[last.Month()-1], last.Year())
		case from.Month() != last.Month():
			return fmt.Sprintf("%d %s – %d %s %d", from.Day(), monthsGen[from.Month()-1],
				last.Day(), monthsGen[last.Month()-1], last.Year())
		default:
			return fmt.Sprintf("%d–%d %s %d", from.Day(), last.Day(), monthsGen[from.Month()-1], from.Year())
		}
	case "month":
		return fmt.Sprintf("%s %d", monthsNom[from.Month()-1], from.Year())
	}
	return fmt.Sprintf("%d год", from.Year())
}

func (s *Store) Total(extra []Play) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.viewLocked(extra)
	return len(s.plays) + len(v.extra)
}

func (s *Store) Top(kind, periodKind string, offset int, now time.Time, extra []Play) (Top, error) {
	if offset > 0 || periodKind == "all" {
		offset = 0
	}
	var from, to time.Time
	lo, hi := never, always
	if periodKind != "all" {
		var err error
		if from, to, err = Bounds(periodKind, offset, now); err != nil {
			return Top{}, err
		}
		lo, hi = from.Unix(), to.Unix()
	}
	pick, err := picker(kind)
	if err != nil {
		return Top{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.viewLocked(extra)

	counts := map[int32]int{}
	total := 0
	v.each(lo, hi, func(r rec) {
		total++
		if id := pick(r); id >= 0 {
			counts[id]++
		}
	})

	top := Top{Kind: kind, Total: total, Items: []TopItem{}}
	first, hasFirst := v.first()
	if periodKind == "all" {
		top.Period = Period{Kind: periodKind, Label: "за всё время", To: now}
		if hasFirst {
			since := time.Unix(first, 0).In(now.Location())
			top.Period.From, top.Period.Since = since, &since
		}
	} else {
		top.Period = Period{
			Kind: periodKind, Offset: offset, From: from, To: to,
			Label: label(periodKind, from, to), Live: offset == 0, HasNext: offset < 0,
			HasPrev: hasFirst && first < from.Unix(),
		}
	}

	type kv struct {
		id    int32
		n     int
		label string
	}
	list := make([]kv, 0, len(counts))
	for id, n := range counts {
		list = append(list, kv{id, n, strings.ToLower(s.itemLabel(kind, id))})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return list[i].label < list[j].label
	})
	for i, e := range list {
		if i == TopSize {
			break
		}
		item := s.item(kind, e.id)
		item.Rank, item.Count = i+1, e.n
		top.Items = append(top.Items, item)
	}
	return top, nil
}

func picker(kind string) (func(rec) int32, error) {
	switch kind {
	case "artists":
		return func(r rec) int32 { return r.artist }, nil
	case "tracks":
		return func(r rec) int32 { return r.track }, nil
	case "albums":
		return func(r rec) int32 { return r.album }, nil
	}
	return nil, fmt.Errorf("неизвестный вид топа %q", kind)
}

func (s *Store) item(kind string, id int32) TopItem {
	switch kind {
	case "artists":
		return TopItem{Artist: s.artists.list[id].text}
	case "tracks":
		t := s.tracks.list[id]
		return TopItem{Artist: s.artists.list[t.artist].text, Title: t.text}
	default:
		a := s.albums.list[id]
		return TopItem{Artist: s.artists.list[a.artist].text, Album: a.text}
	}
}

func (s *Store) itemLabel(kind string, id int32) string {
	it := s.item(kind, id)
	return it.Artist + "\x00" + it.Title + it.Album
}

func (s *Store) Card(kind, artist, title, album string, now time.Time, extra []Play) (Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.viewLocked(extra)

	card := Card{Kind: kind, Artist: strings.TrimSpace(artist), Title: strings.TrimSpace(title), Album: strings.TrimSpace(album)}
	aKey := norm(artist)
	aID, aOK := s.artists.find(aKey)
	var match func(rec) bool
	switch kind {
	case "track":
		id, ok := s.tracks.find(aKey + "\x00" + norm(title))
		if !ok {
			return card, nil
		}
		match = func(r rec) bool { return r.track == id }
		card.Artist, card.Title = s.artists.list[s.tracks.list[id].artist].text, s.tracks.list[id].text
	case "album":
		id, ok := s.albums.find(aKey + "\x00" + norm(album))
		if !ok {
			return card, nil
		}
		match = func(r rec) bool { return r.album == id }
		card.Artist, card.Album = s.artists.list[s.albums.list[id].artist].text, s.albums.list[id].text
	case "artist":
		if !aOK {
			return card, nil
		}
		match = func(r rec) bool { return r.artist == aID }
		card.Artist = s.artists.list[aID].text
	default:
		return card, fmt.Errorf("неизвестный вид карточки %q", kind)
	}

	periods := [3]struct {
		from, to int64
		n        *int
	}{}
	for i, pk := range []string{"week", "month", "year"} {
		f, t, _ := Bounds(pk, 0, now)
		periods[i].from, periods[i].to = f.Unix(), t.Unix()
	}
	periods[0].n, periods[1].n, periods[2].n = &card.Week, &card.Month, &card.Year

	first, last := int64(0), int64(0)
	var lastAlbum int32 = -1
	var lastAlbumTs int64
	tracks := map[int32]int{}
	albums := map[int32]int{}
	artistCount := 0
	v.each(never, always, func(r rec) {
		if aOK && r.artist == aID {
			artistCount++
		}
		if !match(r) {
			return
		}
		card.Count++
		if first == 0 || r.ts < first {
			first = r.ts
		}
		if r.ts > last {
			last = r.ts
		}
		for _, p := range periods {
			if r.ts >= p.from && r.ts < p.to {
				*p.n++
			}
		}
		if r.album >= 0 && r.ts >= lastAlbumTs {
			lastAlbum, lastAlbumTs = r.album, r.ts
		}
		tracks[r.track]++
		if r.album >= 0 {
			albums[r.album]++
		}
	})
	if card.Count == 0 {
		return card, nil
	}
	f, l := time.Unix(first, 0), time.Unix(last, 0)
	card.First, card.Last = &f, &l

	switch kind {
	case "track":
		card.ArtistCount = artistCount
		if lastAlbum >= 0 {
			card.Album = s.albums.list[lastAlbum].text
			v.each(never, always, func(r rec) {
				if r.album == lastAlbum {
					card.AlbumCount++
				}
			})
		}
	case "album":
		card.ArtistCount = artistCount
		card.Tracks = s.topOf(tracks, "tracks", TopSize)
	case "artist":
		card.Tracks = s.topOf(tracks, "tracks", 5)
		card.Albums = s.topOf(albums, "albums", 5)
	}
	return card, nil
}

func (s *Store) topOf(counts map[int32]int, kind string, n int) []CardItem {
	ids := make([]int32, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if counts[ids[i]] != counts[ids[j]] {
			return counts[ids[i]] > counts[ids[j]]
		}
		return strings.ToLower(s.itemLabel(kind, ids[i])) < strings.ToLower(s.itemLabel(kind, ids[j]))
	})
	out := make([]CardItem, 0, n)
	for _, id := range ids {
		if len(out) == n {
			break
		}
		it := s.item(kind, id)
		out = append(out, CardItem{Artist: it.Artist, Title: it.Title, Album: it.Album, Count: counts[id]})
	}
	return out
}

type Rank struct {
	Kind   string `json:"kind"`
	Period string `json:"period"`
	Rank   int    `json:"rank"`
}

var rankPeriods = [...]string{"week", "month", "year"}

func (s *Store) Ranks(artist, title, album string, now time.Time, extra []Play) []Rank {
	if strings.TrimSpace(artist) == "" {
		return nil
	}
	k := keysOf(Play{Artist: artist, Title: title, Album: album})
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.viewLocked(extra)

	type want struct {
		kind string
		id   int32
		pick func(rec) int32
	}
	var wants []want
	add := func(kind string, t *table, key string) {
		if key == "" {
			return
		}
		if id, ok := t.find(key); ok {
			pick, _ := picker(kind)
			wants = append(wants, want{kind, id, pick})
		}
	}
	if strings.TrimSpace(title) != "" {
		add("tracks", &s.tracks, k.track)
	}
	add("artists", &s.artists, k.artist)
	add("albums", &s.albums, k.album)
	if len(wants) == 0 {
		return nil
	}

	var out []Rank
	for _, period := range rankPeriods {
		from, to, err := Bounds(period, 0, now)
		if err != nil {
			continue
		}
		counts := make([]map[int32]int, len(wants))
		for i := range counts {
			counts[i] = map[int32]int{}
		}
		v.each(from.Unix(), to.Unix(), func(r rec) {
			for i, w := range wants {
				if id := w.pick(r); id >= 0 {
					counts[i][id]++
				}
			}
		})
		for i, w := range wants {
			n := counts[i][w.id]
			if n == 0 {
				continue
			}
			mine := strings.ToLower(s.itemLabel(w.kind, w.id))
			rank := 1
			for id, c := range counts[i] {
				if c > n || (c == n && id != w.id && strings.ToLower(s.itemLabel(w.kind, id)) < mine) {
					rank++
				}
			}
			if rank <= TopSize {
				out = append(out, Rank{Kind: strings.TrimSuffix(w.kind, "s"), Period: period, Rank: rank})
			}
		}
	}
	return out
}

func (s *Store) PlayCounts(artists []string, tracks []Play, extra []Play) ([]int, []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.viewLocked(extra)
	byArtist := map[int32][]int{}
	for i, a := range artists {
		if id, ok := s.artists.find(norm(a)); ok {
			byArtist[id] = append(byArtist[id], i)
		}
	}
	byTrack := map[int32][]int{}
	for i, p := range tracks {
		if id, ok := s.tracks.find(keysOf(p).track); ok {
			byTrack[id] = append(byTrack[id], i)
		}
	}
	ac, tc := make([]int, len(artists)), make([]int, len(tracks))
	if len(byArtist) == 0 && len(byTrack) == 0 {
		return ac, tc
	}
	v.each(math.MinInt64, math.MaxInt64, func(r rec) {
		for _, i := range byArtist[r.artist] {
			ac[i]++
		}
		for _, i := range byTrack[r.track] {
			tc[i]++
		}
	})
	return ac, tc
}
