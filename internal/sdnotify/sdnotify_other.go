//go:build !linux

package sdnotify

import "time"

func Enabled() bool { return false }

func Ready() {}

func Status(string, ...any) {}

func Stopping() {}

func Watchdog() {}

func WatchdogInterval() time.Duration { return 0 }
