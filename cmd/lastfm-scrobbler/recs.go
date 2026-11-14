package main

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lastfm"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/library"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/ui"
)

const (
	recsFresh    = 24 * time.Hour
	recsRetry    = 2 * time.Minute
	recsLimit    = 20 * time.Second
	recsKeep     = 100
	recsShown    = 5
	recsAsk      = 30
	recsPerActor = 2
)

type recsCache struct {
	mu    sync.Mutex
	known map[string]*recsEntry
	last  [2]string
	busy  sync.WaitGroup
}

type recsEntry struct {
	recs    ui.Recs
	pending bool
	next    time.Time
}

func emptyRecs(state, artist, title string) ui.Recs {
	return ui.Recs{State: state, Artist: artist, Title: title, Artists: []ui.RecArtist{}, Tracks: []ui.RecTrack{}}
}

func (b *uiBackend) Recs() ui.Recs {
	s := b.svc
	artist, title := "", ""
	if sn, ok := s.activeSnapshot(); ok && sn.Track.Valid() {
		artist, title = sn.Track.Artist(), sn.Track.Title
	}
	c := &s.recs
	c.mu.Lock()
	defer c.mu.Unlock()
	if artist == "" {
		artist, title = c.last[0], c.last[1]
		if artist == "" {
			return emptyRecs(ui.RecsNone, "", "")
		}
	}
	c.last = [2]string{artist, title}
	key := lovedKey("", artist, title)
	e := c.known[key]
	if e == nil {
		if c.known == nil {
			c.known = map[string]*recsEntry{}
		}
		if len(c.known) >= recsKeep {
			for k, old := range c.known {
				if !old.pending {
					delete(c.known, k)
				}
			}
		}
		e = &recsEntry{recs: emptyRecs(ui.RecsLoading, artist, title)}
		c.known[key] = e
	}
	if !e.pending && !time.Now().Before(e.next) {
		e.pending = true
		c.busy.Add(1)
		go s.fetchRecs(key, artist, title)
	}
	return e.recs
}

func (s *service) fetchRecs(key, artist, title string) {
	defer s.recs.busy.Done()
	out := emptyRecs(ui.RecsFound, artist, title)
	if client := s.newClient(s.config(), ""); client == nil {
		out.State = ui.RecsNoKeys
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), recsLimit)
		artists, errA := client.SimilarArtists(ctx, artist, recsAsk)
		tracks, errT := client.SimilarTracks(ctx, artist, title, recsAsk)
		cancel()
		if errA != nil && errT != nil {
			out.State = ui.RecsError
			s.log.Debug("не удалось получить рекомендации Last.fm", "track", artist+" — "+title, "err", errA)
		} else {
			out.Artists, out.Tracks = s.rankRecs(artist, artists, tracks)
		}
	}
	s.recs.mu.Lock()
	defer s.recs.mu.Unlock()
	e := s.recs.known[key]
	if e == nil {
		return
	}
	e.pending = false
	switch out.State {
	case ui.RecsFound:
		e.recs, e.next = out, time.Now().Add(recsFresh)
	default:
		e.next = time.Now().Add(recsRetry)
		if e.recs.State != ui.RecsFound {
			e.recs = out
		}
	}
}

func (s *service) rankRecs(current string, artists []lastfm.SimilarArtist, tracks []lastfm.SimilarTrack) ([]ui.RecArtist, []ui.RecTrack) {
	lib, extra := s.stats()
	names := make([]string, len(artists))
	for i, a := range artists {
		names[i] = a.Name
	}
	songs := make([]library.Play, len(tracks))
	for i, t := range tracks {
		songs[i] = library.Play{Artist: t.Artist, Title: t.Title}
	}
	ac, tc := lib.PlayCounts(names, songs, extra)

	var fresh, heard []ui.RecArtist
	for i, a := range artists {
		r := ui.RecArtist{Name: a.Name, Match: a.Match, Plays: ac[i]}
		if r.Plays == 0 {
			fresh = append(fresh, r)
		} else {
			heard = append(heard, r)
		}
	}
	outA := append(fresh, heard...)
	if len(outA) > recsShown {
		outA = outA[:recsShown]
	}

	self := normName(current)
	var other, same, known []ui.RecTrack
	for i, t := range tracks {
		r := ui.RecTrack{Title: t.Title, Artist: t.Artist, Match: t.Match, Plays: tc[i]}
		switch {
		case r.Plays > 0:
			known = append(known, r)
		case normName(t.Artist) == self:
			same = append(same, r)
		default:
			other = append(other, r)
		}
	}
	return outA, pickTracks(append(append(other, same...), known...))
}

func pickTracks(list []ui.RecTrack) []ui.RecTrack {
	out := []ui.RecTrack{}
	per := map[string]int{}
	used := make([]bool, len(list))
	for pass := 0; pass < 2 && len(out) < recsShown; pass++ {
		for i, t := range list {
			if used[i] || len(out) == recsShown {
				continue
			}
			who := normName(t.Artist)
			if pass == 0 && per[who] >= recsPerActor {
				continue
			}
			used[i] = true
			per[who]++
			out = append(out, t)
		}
	}
	return out
}

func normName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func lastfmURL(artist, track string) string {
	u := "https://www.last.fm/music/" + url.QueryEscape(strings.TrimSpace(artist))
	if track = strings.TrimSpace(track); track != "" {
		u += "/_/" + url.QueryEscape(track)
	}
	return u
}

func (b *uiBackend) OpenLastfm(artist, track string) error {
	if strings.TrimSpace(artist) == "" || len(artist) > 500 || len(track) > 500 {
		return errors.New("неизвестный исполнитель")
	}
	return openURL(lastfmURL(artist, track))
}
