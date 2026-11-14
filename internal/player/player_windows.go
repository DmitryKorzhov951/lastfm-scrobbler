//go:build windows

package player

import "github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/gsmtc"

func newWatcher(opts Options) (Watcher, error) {
	return gsmtc.NewWatcher(gsmtc.Options{
		Poll:  opts.Poll,
		Allow: opts.Allow,
		Deny:  opts.Deny,
		Log:   opts.Log,
	})
}
