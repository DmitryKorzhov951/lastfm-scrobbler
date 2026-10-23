//go:build !windows

package player

import (
	"context"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/mpris"
)

const Source = "MPRIS2 (шина D-Bus)"

const PositionReliable = true

func newWatcher(opts Options) (Watcher, error) {
	return mpris.NewWatcher(mpris.Options{
		Poll:  opts.Poll,
		Allow: opts.Allow,
		Deny:  opts.Deny,
		Log:   opts.Log,
	})
}

func listPlayers(ctx context.Context) ([]media.State, error) {
	return mpris.ListPlayers(ctx)
}
