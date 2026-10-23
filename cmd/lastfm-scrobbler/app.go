package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/atomicfile"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/ui"
)

const exitNoWebView = 3

var (
	errNoWebView   = errors.New("не найден WebView2 Runtime — встроенный движок Edge для окон программ")
	errPageTimeout = errors.New("страница не появилась во встроенном окне")
	errNoLoader    = errors.New("рядом с программой нет WebView2Loader.dll — он должен лежать в той же папке, что и exe")
)

func cmdApp(ctx context.Context, e *env, args []string, trayOnly bool) error {
	fs := flag.NewFlagSet("app", flag.ContinueOnError)
	noTray := fs.Bool("no-tray", false, "без значка в трее (для отладки)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "lastfm-scrobbler [-tray] app [-no-tray]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	release, err := acquireInstance()
	if err != nil {
		if u := runningWindowURL(); u != "" {
			return showRunningWindow(u)
		}
		return err
	}
	defer release()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-watchStop(ctx):
			e.log.Info("получен запрос на остановку")
			cancel()
		case <-ctx.Done():
		}
	}()

	svc, err := newService(e, false)
	if err != nil {
		return err
	}
	srv, err := ui.Listen(&uiBackend{svc: svc, env: e}, "127.0.0.1:0", e.log)
	if err != nil {
		return fmt.Errorf("запуск окна: %w", err)
	}
	go func() {
		if err := srv.Serve(); err != nil {
			e.log.Error("сервер окна остановился", "err", err)
		}
	}()
	defer srv.Close()

	show := watchShow(ctx)

	if path, err := config.WindowPath(); err == nil {
		if err := atomicfile.Write(path, []byte(srv.URL()+"\n")); err != nil {
			e.log.Warn("не удалось сохранить адрес окна", "err", err)
		}
		defer os.Remove(path)
	}

	open := func() {
		if err := openWindow(srv.URL()); err != nil {
			e.log.Error("не удалось открыть окно", "err", err)
		}
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-show:
				open()
			}
		}
	}()

	runErr := make(chan error, 1)
	go func() {
		runErr <- svc.Run(ctx, nil)
		cancel()
	}()

	if !trayOnly || !svc.sub.Status().LoggedIn {
		open()
	}

	if *noTray {
		fmt.Println(srv.URL())
		<-ctx.Done()
	} else {
		runTray(ctx, svc, e, open)
		cancel()
	}
	return <-runErr
}

func runningWindowURL() string {
	path, err := config.WindowPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	u := strings.TrimSpace(string(data))
	if !strings.HasPrefix(u, "http://127.0.0.1:") {
		return ""
	}
	base, _, _ := strings.Cut(u, "#")
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(base)
	if err != nil {
		return ""
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	return u
}

func cmdOpen(ctx context.Context, e *env, args []string) error {
	if u := runningWindowURL(); u != "" {
		return showRunningWindow(u)
	}
	return cmdApp(ctx, e, args, false)
}
