//go:build !windows

package main

import (
	"context"
	"errors"
	"os/exec"
	"runtime"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
)

const platformName = "Linux"

const platformCommands = `  run       запустить скробблер (команда по умолчанию)
`

const platformExamples = `  lastfm-scrobbler run                  запустить в переднем плане
  systemctl --user enable --now lastfm-scrobbler.service
`

var errUseSystemd = errors.New("в Linux фоновой работой управляет systemd: " +
	"systemctl --user start|stop|enable lastfm-scrobbler.service")

func cmdStart(*env, []string) error     { return errUseSystemd }
func cmdStop(*env, []string) error      { return errUseSystemd }
func cmdAutostart(*env, []string) error { return errUseSystemd }

func setupBackground() error { return errUseSystemd }

func acquireInstance() (func(), error) { return func() {}, nil }

func watchStop(context.Context) <-chan struct{} { return nil }

func platformStatus() [][2]string { return nil }

func openURL(url string) error {
	openers := [][]string{{"xdg-open"}, {"gio", "open"}, {"kde-open"}, {"firefox"}}
	if runtime.GOOS == "darwin" {
		openers = [][]string{{"open"}}
	}
	for _, o := range openers {
		path, err := exec.LookPath(o[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(path, append(o[1:], url)...)
		if err := cmd.Start(); err == nil {
			go cmd.Wait()
			return nil
		}
	}
	return errors.New("не найдено средство открытия ссылок")
}

func openWindow(url string) error {
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "microsoft-edge"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, "--app="+url, "--window-size=980,720")
		if err := cmd.Start(); err == nil {
			go cmd.Wait()
			return nil
		}
	}
	return openURL(url)
}

func windowRunning() bool { return false }

func showRunningWindow(url string) error { return openWindow(url) }

func watchShow(context.Context) <-chan struct{} { return nil }

func cmdWindow([]string) error {
	return errors.New("команда window есть только в Windows")
}

const autostartSupported = false

func autostartEnabled() bool        { return false }
func setAutostart(*env, bool) error { return errUseSystemd }

func logDir() string {
	dir, err := config.StateDir()
	if err != nil {
		return "journalctl --user -u lastfm-scrobbler"
	}
	return dir
}

func openFolder(path string) error { return openURL(path) }
