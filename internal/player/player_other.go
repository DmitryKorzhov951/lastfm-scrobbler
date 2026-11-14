//go:build !windows

package player

import "errors"

func newWatcher(Options) (Watcher, error) {
	return nil, errors.New("слежение за плеерами работает только в Windows")
}
