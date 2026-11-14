package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/cover"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/version"
)

func newCovers(log *slog.Logger) *cover.Store {
	dir, err := config.CoverDir()
	if err != nil {
		log.Warn("обложки не будут сохраняться на диск", "err", err)
		dir = ""
	}
	s := cover.New(dir, log)
	s.HTTP = &http.Client{Timeout: 20 * time.Second}
	s.UserAgent = version.UserAgent()
	return s
}

func (s *service) findCover(ctx context.Context, artist, title, album string) (string, error) {
	client := s.newClient(s.config(), "")
	if client == nil {
		return "", nil
	}
	return client.CoverURL(ctx, artist, title, album)
}
