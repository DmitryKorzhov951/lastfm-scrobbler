package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/history"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lastfm"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/library"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/queue"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/ui"
)

type lastfmSource struct {
	client *lastfm.Client
}

func (s lastfmSource) RecentTracks(ctx context.Context, user string, page int, from, to time.Time) (library.Page, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	p, err := s.client.RecentTracks(ctx, user, page, lastfm.MaxRecentLimit, from, to)
	if err != nil {
		return library.Page{}, err
	}
	out := library.Page{TotalPages: p.TotalPages, Total: p.Total, Plays: make([]library.Play, 0, len(p.Tracks))}
	for _, t := range p.Tracks {
		if t.NowPlaying || t.At.IsZero() {
			continue
		}
		out.Plays = append(out.Plays, library.Play{At: t.At, Artist: t.Artist, Title: t.Title, Album: t.Album})
	}
	return out, nil
}

func libraryErrorText(err error) string {
	if apiErr, ok := lastfm.AsError(err); ok {
		switch apiErr.Code {
		case lastfm.ErrCodeLoginRequired:
			return "Last.fm не отдаёт историю: в настройках приватности на сайте скрыты недавние прослушивания"
		case lastfm.ErrCodeRateLimit:
			return "Last.fm просит подождать: слишком много запросов"
		case lastfm.ErrCodeInvalidAPIKey:
			return "Last.fm не узнал API key — проверьте ключ"
		}
		if apiErr.Code != 0 {
			return "Last.fm ответил ошибкой: " + apiErr.Message
		}
		return "Last.fm временно недоступен"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "Last.fm не ответил вовремя"
	}
	return "нет связи с Last.fm"
}

func openLibrary(e *env) (*library.Store, error) {
	plays, meta, err := config.LibraryPaths()
	if err != nil {
		return nil, err
	}
	lib, err := library.Open(plays, meta)
	if err != nil && lib == nil {
		e.log.Warn("история прослушиваний Last.fm недоступна — работаю без неё", "err", err)
		lib, err = library.Open("", "")
	}
	return lib, err
}

func (s *service) setLibraryAccount(cfg config.Config, sess config.Session) {
	if s.libSync == nil {
		return
	}
	client := s.newClient(cfg, sess.Key)
	if client == nil || sess.Key == "" || sess.Username == "" {
		s.libSync.SetAccount(nil, "")
		return
	}
	s.libSync.SetAccount(lastfmSource{client: client}, sess.Username)
}

const localOverlap = 15 * 24 * time.Hour

func (s *service) localPlays() []library.Play {
	var since time.Time
	if newest := s.lib.Newest(); !newest.IsZero() {
		since = newest.Add(-localOverlap)
	}
	entries := s.hist.Since(since)
	out := make([]library.Play, 0, len(entries))
	for _, e := range entries {
		if e.Status == history.Ignored {
			continue
		}
		out = append(out, library.Play{At: e.Started, Artist: e.Artist, Title: e.Title, Album: e.Album})
	}
	return out
}

const fromLastfm = "lastfm"

func (s *service) recentScrobbles(limit int) []ui.Scrobble {
	local := s.hist.Recent(limit)
	var remote []library.Play
	if !s.localMode() {
		remote = s.lib.Recent(limit + len(local))
	}
	own := make(map[int64]bool, len(local))
	for _, e := range local {
		own[e.Started.Unix()] = true
	}
	out := make([]ui.Scrobble, 0, limit)
	i, j := 0, 0
	for len(out) < limit && (i < len(local) || j < len(remote)) {
		if j < len(remote) && own[remote[j].At.Unix()] {
			j++
			continue
		}
		if i < len(local) && (j >= len(remote) || !local[i].Started.Before(remote[j].At)) {
			e := local[i]
			i++
			out = append(out, ui.Scrobble{
				ID: e.ID, Started: e.Started, Artist: e.Artist, Title: e.Title, Album: e.Album,
				Status: string(e.Status), Reason: e.Reason,
			})
			continue
		}
		p := remote[j]
		j++
		out = append(out, ui.Scrobble{Started: p.At, Artist: p.Artist, Title: p.Title, Album: p.Album, Status: fromLastfm})
	}
	return out
}

func openArchive(e *env, hist *history.Store) (*library.Store, error) {
	path, err := config.LocalPath()
	if err != nil {
		return nil, err
	}
	archive, err := library.Open(path, "")
	if err != nil && archive == nil {
		e.log.Warn("прослушивания на этом компьютере недоступны — работаю без них", "err", err)
		archive, err = library.Open("", "")
	}
	if archive != nil {
		var plays []library.Play
		for _, en := range hist.Since(time.Time{}) {
			if en.Status != history.Ignored {
				plays = append(plays, library.Play{At: en.Started, Artist: en.Artist, Title: en.Title, Album: en.Album})
			}
		}
		if _, addErr := archive.Add(plays); addErr != nil && err == nil {
			err = addErr
		}
	}
	return archive, err
}

func playsOf(entries []history.Entry) []library.Play {
	out := make([]library.Play, 0, len(entries))
	for _, e := range entries {
		out = append(out, library.Play{At: e.Started, Artist: e.Artist, Title: e.Title, Album: e.Album})
	}
	return out
}

func (s *service) localMode() bool { return s.account().Key == "" }

func (s *service) stats() (*library.Store, []library.Play) {
	if s.localMode() {
		return s.archive, nil
	}
	return s.lib, s.localPlays()
}

func (s *service) keepLocal() {
	s.sub.Exclusive(s.moveQueueLocal)
}

func (s *service) moveQueueLocal() {
	items := s.queue.Items()
	if len(items) == 0 {
		return
	}
	ids := itemIDs(items)
	if err := s.hist.SetStatus(history.Local, "", ids...); err != nil {
		s.log.Error("не удалось записать историю", "err", err)
	}
	if _, err := s.queue.Remove(ids...); err != nil {
		s.log.Error("не удалось очистить очередь", "err", err)
		return
	}
	s.log.Info("очередь без входа сохранена на этом компьютере", "count", len(ids))
}

func (s *service) sendRecentLocal() {
	var items []queue.Item
	var ids []int64
	for _, e := range s.hist.Since(time.Now().Add(-maxScrobbleAge)) {
		if e.Status != history.Local {
			continue
		}
		items = append(items, queue.Item{ID: e.ID, QueuedAt: time.Now().UTC(), Scrobble: lastfm.Scrobble{
			Track:     lastfm.Track{Artist: e.Artist, Title: e.Title, Album: e.Album},
			Timestamp: e.Started,
		}})
		ids = append(ids, e.ID)
	}
	if len(items) == 0 {
		return
	}
	if err := s.queue.PushItems(items); err != nil {
		s.log.Error("не удалось поставить прослушивания в очередь", "err", err)
		return
	}
	if err := s.hist.SetStatus(history.Queued, "", ids...); err != nil {
		s.log.Error("не удалось записать историю", "err", err)
	}
	s.sub.RequestFlush()
	s.log.Info("прослушивания за последние 14 дней отправятся в Last.fm", "count", len(items))
}

type rankCache struct {
	key string
	at  time.Time
	val []library.Rank
}

const rankTTL = time.Minute

func (s *service) ranks(artist, title, album string) []library.Rank {
	local := s.localMode()
	lib := s.lib
	if local {
		lib = s.archive
	}
	key := fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d\x00%t", artist, title, album, lib.Len(), s.hist.LastID(), local)
	s.ranksMu.Lock()
	c := s.ranked
	s.ranksMu.Unlock()
	if c.key == key && time.Since(c.at) < rankTTL {
		return c.val
	}
	src, extra := s.stats()
	val := src.Ranks(artist, title, album, time.Now(), extra)
	s.ranksMu.Lock()
	s.ranked = rankCache{key: key, at: time.Now(), val: val}
	s.ranksMu.Unlock()
	return val
}
