package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/scrobbler"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/sdnotify"
)

func cmdRun(ctx context.Context, e *env, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "ничего не отправлять, только писать в лог")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "lastfm-scrobbler run [-dry-run]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if _, err := e.client(); err != nil {
		return err
	}
	if e.session.Key == "" && !*dryRun {
		return fmt.Errorf("нет ключа сессии — выполните «lastfm-scrobbler auth»")
	}

	release, err := acquireInstance()
	if err != nil {
		return err
	}
	defer release()

	ctx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	go func() {
		select {
		case <-watchStop(ctx):
			e.log.Info("получен запрос на остановку")
			cancelRun()
		case <-ctx.Done():
		}
	}()

	svc, err := newService(e, *dryRun)
	if err != nil {
		return err
	}
	return svc.Run(ctx, func() {
		sdnotify.Ready()
		sdnotify.Status("вход выполнен как %s; в очереди %d", displayUser(e.session.Username), svc.queue.Len())
	})
}

func displayUser(name string) string {
	if name == "" {
		return "(неизвестно)"
	}
	return name
}

func currentTrackLine(engine *scrobbler.Engine) string {
	for _, s := range engine.Snapshots() {
		if s.Active && s.Status == media.Playing {
			return fmt.Sprintf("%s — %s", s.Identity, s.Track.String())
		}
	}
	return "воспроизведение не идёт"
}
