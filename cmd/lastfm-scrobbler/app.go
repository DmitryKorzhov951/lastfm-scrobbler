package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/atomicfile"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/ui"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/version"
)

const exitNoWebView = 3

var (
	errNoWebView   = errors.New("в Windows не найден WebView2 Runtime — движок, которым программы рисуют свои окна")
	errPageTimeout = errors.New("окно не загрузилось за 30 секунд")
)

func cmdApp(ctx context.Context, e *env, trayOnly bool) error {
	release, err := acquireInstance()
	if err != nil {
		u := runningWindowURL()
		if u != "" && sameBuild(u) {
			return showRunningWindow(u)
		}
		e.log.Info("работает другая копия программы — останавливаю её и запускаюсь вместо неё")
		if r := replaceRunning(); r != nil {
			release = r
		} else if u != "" {
			return showRunningWindow(u)
		} else {
			return err
		}
	}
	defer release()
	refreshAutostart(e)

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

	svc, err := newService(e)
	if err != nil {
		return err
	}
	srv, err := ui.Listen(&uiBackend{svc: svc, env: e}, "127.0.0.1:0", e.log)
	if err != nil {
		return fmt.Errorf("запуск окна: %w", err)
	}
	srv.SetAbout(currentAbout())
	go func() {
		if err := srv.Serve(); err != nil {
			e.log.Error("сервер окна остановился", "err", err)
		}
	}()
	defer srv.Close()
	svc.startPhone(&uiBackend{svc: svc, env: e})
	defer svc.stopPhone()

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

	if !trayOnly {
		open()
	}

	runTray(ctx, svc, e, open)
	cancel()
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

type appAbout struct {
	Version string `json:"version"`
	Dir     string `json:"dir"`
}

func currentAbout() appAbout {
	a := appAbout{Version: version.Version}
	if exe, err := os.Executable(); err == nil {
		a.Dir = filepath.Dir(exe)
	}
	return a
}

func sameBuild(windowURL string) bool {
	base, _, _ := strings.Cut(windowURL, "#")
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(strings.TrimSuffix(base, "/") + "/about")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var a appAbout
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&a); err != nil {
		return false
	}
	cur := currentAbout()
	return a.Version == cur.Version && strings.EqualFold(filepath.Clean(a.Dir), filepath.Clean(cur.Dir))
}
