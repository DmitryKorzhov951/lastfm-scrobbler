package main

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lastfm"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/moments"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/scrobbler"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/ui"
)

func TestВЛюбимыеМоментыИдутТолькоТочныеПозиции(t *testing.T) {
	tr := media.Track{Title: "Песня", Artists: []string{"Тестовая группа"}, Length: 3 * time.Minute}
	live := tr
	live.Live = true
	noLen := tr
	noLen.Length = 0
	snaps := []scrobbler.Snapshot{
		{ID: "a", Status: media.Playing, Track: tr, Position: 42 * time.Second, Synced: true},
		{ID: "b", Status: media.Paused, Track: tr, Position: 10 * time.Second, Synced: true},
		{ID: "c", Status: media.Playing, Track: tr, Position: 5 * time.Second},
		{ID: "d", Status: media.Playing, Track: live, Position: 5 * time.Second, Synced: true},
		{ID: "e", Status: media.Playing, Track: noLen, Position: 5 * time.Second, Synced: true},
		{ID: "f", Status: media.Stopped, Track: tr, Position: 5 * time.Second, Synced: true},
	}
	got := momentSamples(snaps)
	if len(got) != 2 || got[0].Player != "a" || got[0].Position != 42 || !got[0].Playing || got[1].Player != "b" || got[1].Playing {
		t.Errorf("отобраны позиции: %+v", got)
	}
}

func TestЛюбимыйМоментТекущегоТрека(t *testing.T) {
	svc, w, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) { io.WriteString(rw, `{"nowplaying":{}}`) })
	b := &uiBackend{svc: svc}
	st := playing("spotify", "Spotify", "Тестовая группа", "Песня")
	st.Track.Length = 3 * time.Minute
	listen(svc, w, st, 10*time.Second)
	if n := b.State().Now; n == nil || n.Moment != nil {
		t.Fatalf("пока момента нет: %+v", n)
	}
	svc.do(context.Background(), func(*scrobbler.Engine) {
		at := time.Now().Add(-time.Hour)
		for i := 0; i < 3; i++ {
			for _, part := range [][2]float64{{0, 70}, {40, 80}} {
				for pos := part[0]; pos <= part[1]; pos++ {
					svc.moments.Observe([]moments.Sample{{Player: "x", Artist: "Тестовая группа", Title: "Песня", Length: 180, Position: pos, Playing: true}}, at)
					at = at.Add(time.Second)
				}
			}
			svc.moments.Observe(nil, at)
		}
	})
	n := b.State().Now
	if n == nil || n.Moment == nil || n.Moment.Manual || n.Moment.Start < 39 || n.Moment.End > 72 {
		t.Fatalf("переслушанное место — любимый момент: %+v", n.Moment)
	}
	if err := b.SetMoment(ui.MomentRequest{Artist: "Тестовая группа", Title: "Песня", Length: 180, Start: 120, End: 135}); err != nil {
		t.Fatal(err)
	}
	if m := b.State().Now.Moment; m == nil || !m.Manual || m.Start != 120 || m.End != 135 {
		t.Errorf("свой момент: %+v", m)
	}
	if err := b.SetMoment(ui.MomentRequest{Artist: "Тестовая группа", Title: "Песня", Length: 180, Start: 170, End: 170.5}); err == nil {
		t.Error("слишком короткий момент принят")
	}
	if err := b.SetMoment(ui.MomentRequest{Artist: "Тестовая группа", Title: "Песня", Clear: true}); err != nil {
		t.Fatal(err)
	}
	if m := b.State().Now.Moment; m == nil || m.Manual {
		t.Errorf("после сброса снова автоматический: %+v", m)
	}
}

type silentSink struct{}

func (silentSink) NowPlaying(lastfm.Track)  {}
func (silentSink) Scrobble(lastfm.Scrobble) {}

func TestПеремоткаВSpotifyСразуДаётМомент(t *testing.T) {
	mom, err := moments.Open(filepath.Join(t.TempDir(), "moments.json"), testLogger())
	if err != nil {
		t.Fatal(err)
	}
	eng := scrobbler.New(scrobbler.Options{IgnoreInitialPosition: true}, silentSink{}, testLogger())
	tr := media.Track{Title: "Песня", Artists: []string{"Тестовая группа"}, Length: 200 * time.Second}
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	repAt, repPos := now, time.Duration(0)
	report := func(pos time.Duration) {
		repAt, repPos = now, pos
		eng.Update(media.State{ID: "spotify", Name: "Spotify", Status: media.Playing, Track: tr, At: now,
			Position: pos, PositionKnown: true, PositionReported: repAt, PositionFresh: true})
	}
	play := func(seconds int) {
		for i := 0; i < seconds; i++ {
			now = now.Add(time.Second)
			eng.Tick(now)
			if i%2 == 1 {
				eng.Update(media.State{ID: "spotify", Name: "Spotify", Status: media.Playing, Track: tr, At: now,
					Position: repPos + now.Sub(repAt), PositionKnown: true, PositionReported: repAt})
			}
			mom.Observe(momentSamples(eng.SnapshotsAt(now)), now)
		}
	}
	report(0)
	play(70)
	if _, ok := mom.Moment("Тестовая группа", "Песня", 200); ok {
		t.Fatal("без перемотки момента нет")
	}
	report(40 * time.Second)
	play(35)
	m, ok := mom.Moment("Тестовая группа", "Песня", 200)
	if !ok || m.Start < 39 || m.Start > 43 || m.End < 68 || m.End > 72 {
		t.Fatalf("одна перемотка назад 1:10 → 0:40 — и момент виден, пока песня играет: %+v, %v", m, ok)
	}
}
