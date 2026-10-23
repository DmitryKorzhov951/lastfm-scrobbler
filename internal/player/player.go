package player

import (
	"context"
	"log/slog"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

type Watcher interface {
	Events() <-chan media.State
	Run(ctx context.Context) error
	Close() error
}

type Options struct {
	Poll  time.Duration
	Allow []string
	Deny  []string
	Log   *slog.Logger
}

func New(opts Options) (Watcher, error) { return newWatcher(opts) }

func List(ctx context.Context) ([]media.State, error) { return listPlayers(ctx) }
