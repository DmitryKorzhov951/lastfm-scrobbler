package main

import (
	"errors"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/moments"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/scrobbler"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/ui"
)

func openMoments(e *env) (*moments.Store, error) {
	path, err := config.MomentsPath()
	if err != nil {
		e.log.Warn("любимые моменты не будут сохраняться", "err", err)
		path = ""
	}
	return moments.Open(path, e.log)
}

func momentSamples(snaps []scrobbler.Snapshot) []moments.Sample {
	out := make([]moments.Sample, 0, len(snaps))
	for _, sn := range snaps {
		if !sn.Synced || !sn.Track.Valid() || sn.Track.Live || sn.Track.Length <= 0 {
			continue
		}
		if sn.Status != media.Playing && sn.Status != media.Paused {
			continue
		}
		out = append(out, moments.Sample{
			Player: sn.ID, Artist: sn.Track.Artist(), Title: sn.Track.Title, Length: sn.Track.Length.Seconds(),
			Position: sn.Position.Seconds(), Playing: sn.Status == media.Playing,
		})
	}
	return out
}

func (s *service) moment(t media.Track) *moments.Moment {
	if !t.Valid() || t.Live || t.Length <= 0 {
		return nil
	}
	if m, ok := s.moments.Moment(t.Artist(), t.Title, t.Length.Seconds()); ok {
		return &m
	}
	return nil
}

func (b *uiBackend) SetMoment(r ui.MomentRequest) error {
	if r.Clear {
		return b.svc.moments.ClearManual(r.Artist, r.Title)
	}
	if r.Artist == "" || r.Title == "" {
		return errors.New("неизвестный трек")
	}
	return b.svc.moments.SetManual(r.Artist, r.Title, r.Length, moments.Moment{Start: r.Start, End: r.End}, time.Now())
}
