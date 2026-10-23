//go:build windows

package player

import (
	"context"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/gsmtc"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

const Source = "системные медиаэлементы Windows (GSMTC)"

const PositionReliable = false

func newWatcher(opts Options) (Watcher, error) {
	return gsmtc.NewWatcher(gsmtc.Options{
		Poll:  opts.Poll,
		Allow: opts.Allow,
		Deny:  opts.Deny,
		Log:   opts.Log,
	})
}

func listPlayers(ctx context.Context) ([]media.State, error) {
	return gsmtc.ListPlayers(ctx)
}
