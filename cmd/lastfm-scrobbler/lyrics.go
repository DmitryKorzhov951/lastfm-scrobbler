package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lyrics"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/ui"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/version"
)

func newLyrics(s *service, log *slog.Logger) *lyrics.Store {
	dir, err := config.LyricsDir()
	if err != nil {
		log.Warn("тексты песен не будут сохраняться на диск", "err", err)
		dir = ""
	}
	agent := version.UserAgent()
	lrclib := &lyrics.Client{HTTP: &http.Client{Timeout: 15 * time.Second}, UserAgent: agent}
	ovh := &lyrics.OVH{HTTP: &http.Client{Timeout: 20 * time.Second}, UserAgent: agent}
	chain := &lyrics.Chain{Sources: []lyrics.Finder{lrclib.Find, ovh.Find}, Fix: s.correctTrack}
	st := lyrics.New(dir, chain.Find, log)
	st.Folder = func() string { return s.config().LyricsPath() }
	return st
}

func (s *service) correctTrack(ctx context.Context, artist, title string) (string, string, error) {
	client := s.newClient(s.config(), "")
	if client == nil {
		return "", "", errors.New("не заданы ключи приложения Last.fm")
	}
	return client.Correct(ctx, artist, title)
}

func lyricsFolderSetting(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", nil
	}
	if !filepath.IsAbs(dir) {
		return "", errors.New(`папку с текстами укажите полным путём, например C:\Music\Lyrics`)
	}
	dir = filepath.Clean(dir)
	if auto := config.DefaultLyricsFolder(); auto != "" && strings.EqualFold(dir, filepath.Clean(auto)) {
		return "", nil
	}
	return dir, nil
}

func (b *uiBackend) openLyricsFolder() error {
	dir := b.svc.config().LyricsPath()
	if dir == "" {
		return errors.New("не удалось определить папку с текстами")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return openFolder(dir)
}

func (b *uiBackend) Lyrics() ui.Lyrics {
	sn, ok := b.svc.activeSnapshot()
	if !ok || !sn.Track.Valid() {
		return ui.Lyrics{State: ui.NoTrack}
	}
	r := b.svc.lyrics.Get(sn.Track)
	return ui.Lyrics{Artist: sn.Track.Artist(), Title: sn.Track.Title, Album: sn.Track.Album, State: r.State, Lyrics: r.Lyrics}
}

func (b *uiBackend) SetClosed(panels []string) error {
	cfg := b.svc.config()
	cfg.ClosedPanels = panels
	return b.svc.ApplyConfig(context.Background(), cfg)
}

func lyricsSearchURL(artist, title string) string {
	return "https://www.google.com/search?q=" + url.QueryEscape(artist+" "+lyrics.CleanTitle(title)+" текст песни")
}

func (b *uiBackend) openLyricsSearch() error {
	sn, ok := b.svc.activeSnapshot()
	if !ok || !sn.Track.Valid() {
		return errors.New("сейчас ничего не играет")
	}
	return openURL(lyricsSearchURL(sn.Track.Artist(), sn.Track.Title))
}
