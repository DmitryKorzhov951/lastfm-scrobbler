package main

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lyrics"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/ui"
)

func TestТекстПесниДляТекущегоТрека(t *testing.T) {
	svc, w, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) { io.WriteString(rw, `{"nowplaying":{}}`) })
	var asked atomic.Int32
	svc.lyrics = lyrics.New(t.TempDir(), func(_ context.Context, q lyrics.Query) (lyrics.Lyrics, error) {
		asked.Add(1)
		if q.Artist != "Charli XCX" || q.Title != "360" || q.Duration != 60 {
			t.Errorf("неожиданный запрос текста: %+v", q)
		}
		return lyrics.Lyrics{Plain: "первая строка\nвторая строка", Source: lyrics.FromLRCLIB}, nil
	}, testLogger())
	b := &uiBackend{svc: svc}
	if l := b.Lyrics(); l.State != ui.NoTrack {
		t.Errorf("без трека: %+v", l)
	}
	listen(svc, w, playing("spotify", "Spotify", "Charli XCX", "360"), 10*time.Second)
	if l := b.Lyrics(); l.State != lyrics.Loading {
		t.Errorf("сначала текст ищется: %+v", l)
	}
	svc.lyrics.Wait()
	l := b.Lyrics()
	if l.State != lyrics.Found || l.Plain != "первая строка\nвторая строка" || l.Artist != "Charli XCX" || l.Title != "360" {
		t.Errorf("найденный текст: %+v", l)
	}
	if asked.Load() != 1 {
		t.Errorf("поиск текста один раз на трек: %d", asked.Load())
	}
	u := lyricsSearchURL("Charli XCX", "360 (feat. Robyn & Yung Lean)")
	if !strings.HasPrefix(u, "https://www.google.com/search?q=") || !strings.Contains(u, "Charli+XCX+360+") || strings.Contains(u, "Robyn") {
		t.Errorf("поиск текста в браузере: %s", u)
	}
}

func TestЗакрытыеОкнаЗапоминаются(t *testing.T) {
	svc, _, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) { io.WriteString(rw, `{"nowplaying":{}}`) })
	b := &uiBackend{svc: svc}
	if st := b.State(); len(st.Closed) != 0 {
		t.Fatalf("по умолчанию все окна открыты: %v", st.Closed)
	}
	if err := b.SetClosed([]string{"lyrics", "recent"}); err != nil {
		t.Fatal(err)
	}
	if st := b.State(); !slices.Equal(st.Closed, []string{"lyrics", "recent"}) {
		t.Errorf("закрытые окна в состоянии: %v", st.Closed)
	}
	cfg, err := config.Load(svc.cfgPath)
	if err != nil || !slices.Equal(cfg.ClosedPanels, []string{"lyrics", "recent"}) {
		t.Errorf("закрытые окна в config.json: %v, %v", cfg.ClosedPanels, err)
	}
	if err := b.SetClosed(nil); err != nil {
		t.Fatal(err)
	}
	if st := b.State(); len(st.Closed) != 0 {
		t.Errorf("окна открылись снова: %v", st.Closed)
	}
}

func TestПапкаСТекстамиВНастройках(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	svc, _, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) { io.WriteString(rw, `{"nowplaying":{}}`) })
	b := &uiBackend{svc: svc}
	auto := filepath.Join(home, "Music", "Lyrics")
	if s := b.Settings(); s.LyricsDir != auto || s.LyricsAuto != auto || svc.lyrics.Folder() != auto {
		t.Fatalf("папка по умолчанию: %q, %q, %q", s.LyricsDir, s.LyricsAuto, svc.lyrics.Folder())
	}
	custom := filepath.Join(t.TempDir(), "Тексты")
	s := b.Settings()
	s.LyricsDir = " " + custom + string(filepath.Separator) + " "
	if err := b.SaveSettings(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := b.Settings().LyricsDir; got != custom || svc.lyrics.Folder() != custom {
		t.Errorf("своя папка: %q, хранилище смотрит в %q", got, svc.lyrics.Folder())
	}
	if cfg, err := config.Load(svc.cfgPath); err != nil || cfg.LyricsFolder != custom {
		t.Errorf("своя папка в config.json: %q, %v", cfg.LyricsFolder, err)
	}
	s.LyricsDir = "Тексты"
	if err := b.SaveSettings(context.Background(), s); err == nil || !strings.Contains(err.Error(), "полным путём") {
		t.Errorf("относительный путь не принимается: %v", err)
	}
	if got := b.Settings().LyricsDir; got != custom {
		t.Errorf("после ошибки папка прежняя: %q", got)
	}
	for _, back := range []string{auto, ""} {
		s.LyricsDir = back
		if err := b.SaveSettings(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if cfg := svc.config(); cfg.LyricsFolder != "" || b.Settings().LyricsDir != auto {
			t.Errorf("%q возвращает папку по умолчанию: %q", back, cfg.LyricsFolder)
		}
	}
}

func TestИменаПесниСLastfmДляПоискаТекста(t *testing.T) {
	svc, _, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("method") == "track.getInfo" {
			if q.Get("autocorrect") != "1" || q.Get("artist") != "charli xcx" {
				t.Errorf("запрос исправления: %s", r.URL.RawQuery)
			}
			io.WriteString(rw, `{"track":{"name":"360","artist":{"name":"Charli XCX"}}}`)
			return
		}
		io.WriteString(rw, `{"nowplaying":{}}`)
	})
	artist, title, err := svc.correctTrack(context.Background(), "charli xcx", "360")
	if err != nil || artist != "Charli XCX" || title != "360" {
		t.Errorf("исправленные имена: %q, %q, %v", artist, title, err)
	}
	cfg := svc.config()
	cfg.APIKey, cfg.APISecret = "", ""
	svc.mu.Lock()
	svc.cfg = cfg
	svc.mu.Unlock()
	if _, _, err := svc.correctTrack(context.Background(), "charli xcx", "360"); err == nil {
		t.Error("без ключей Last.fm исправлять нечем")
	}
}
