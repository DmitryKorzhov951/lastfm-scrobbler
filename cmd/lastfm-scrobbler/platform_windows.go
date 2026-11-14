//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
)

const (
	instanceMutex = `Local\lastfm-scrobbler-instance`
	stopEvent     = `Local\lastfm-scrobbler-stop`
	showEvent     = `Local\lastfm-scrobbler-show`

	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "lastfm-scrobbler"

	logName    = "lastfm-scrobbler.log"
	logMaxSize = 5 << 20
)

func acquireInstance() (func(), error) {
	name, err := windows.UTF16PtrFromString(instanceMutex)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(h)
		return nil, errors.New("скробблер уже запущен")
	}
	if err != nil {
		return nil, fmt.Errorf("создание мьютекса экземпляра: %w", err)
	}
	return func() { windows.CloseHandle(h) }, nil
}

func watchStop(ctx context.Context) <-chan struct{} {
	name, err := windows.UTF16PtrFromString(stopEvent)
	if err != nil {
		return nil
	}
	h, err := windows.CreateEvent(nil, 1, 0, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil
	}
	windows.ResetEvent(h)

	ch := make(chan struct{})
	go func() {
		defer windows.CloseHandle(h)
		for {
			ev, err := windows.WaitForSingleObject(h, 500)
			if err == nil && ev == windows.WAIT_OBJECT_0 {
				close(ch)
				return
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()
	return ch
}

func requestStop() error { return signalEvent(stopEvent) }

func signalEvent(event string) error {
	name, err := windows.UTF16PtrFromString(event)
	if err != nil {
		return err
	}
	h, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, name)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.SetEvent(h)
}

func logPath() string {
	dir, err := config.StateDir()
	if err != nil {
		return logName
	}
	return filepath.Join(dir, logName)
}

func setupBackground() error {
	windows.NewLazySystemDLL("kernel32.dll").NewProc("FreeConsole").Call()

	path := logPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if info, err := os.Stat(path); err == nil && info.Size() > logMaxSize {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	os.Stdout = f
	os.Stderr = f
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func autostartCommandLine() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	return `"` + self + `" -tray`, nil
}

const autostartSupported = true

func autostartEnabled() bool { return autostartValue() != "" }

func setAutostart(e *env, on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("открытие раздела автозапуска: %w", err)
	}
	defer k.Close()
	if !on {
		if err := k.DeleteValue(runValueName); err != nil && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return fmt.Errorf("удаление автозапуска: %w", err)
		}
		return nil
	}
	cmdline, err := autostartCommandLine()
	if err != nil {
		return err
	}
	if err := k.SetStringValue(runValueName, cmdline); err != nil {
		return fmt.Errorf("запись автозапуска: %w", err)
	}
	return nil
}

func autostartValue() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue(runValueName)
	if err != nil {
		return ""
	}
	return v
}

func openURL(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func logDir() string { return filepath.Dir(logPath()) }

func openFolder(path string) error {
	return exec.Command("explorer", path).Start()
}

func edgePath() string {
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		k, err := registry.OpenKey(root, `Software\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := k.GetStringValue("")
		k.Close()
		if err == nil && fileExists(v) {
			return v
		}
	}
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LOCALAPPDATA"} {
		if base := os.Getenv(env); base != "" {
			if p := filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"); fileExists(p) {
				return p
			}
		}
	}
	return ""
}

func openAppWindow(url string) error {
	edge := edgePath()
	if edge == "" {
		return openURL(url)
	}
	profile := filepath.Join(logDir(), "window")
	cmd := exec.Command(edge, "--app="+url, "--window-size=980,720",
		"--user-data-dir="+profile, "--no-first-run", "--no-default-browser-check")
	if err := cmd.Start(); err != nil {
		return openURL(url)
	}
	go cmd.Wait()
	return nil
}

var winProc struct {
	sync.Mutex
	pid int
}

func windowRunning() bool {
	winProc.Lock()
	defer winProc.Unlock()
	return winProc.pid != 0
}

func openWindow(url string) error {
	winProc.Lock()
	defer winProc.Unlock()
	if winProc.pid != 0 {
		raiseAppWindow(uint32(winProc.pid))
		return nil
	}

	cmd, err := windowCommand(url)
	if err != nil {
		go windowFailed(url, "не удалось подготовить запуск окна: "+err.Error())
		return nil
	}
	logPath := windowLogPath()
	var logFile *os.File
	if logPath != "" {
		if f, err := os.Create(logPath); err == nil {
			logFile = f
			cmd.Stderr = f
		}
	}
	allowForeground()
	err = cmd.Start()
	if logFile != nil {
		logFile.Close()
	}
	if err != nil {
		go windowFailed(url, "не удалось запустить процесс окна: "+err.Error())
		return nil
	}
	winProc.pid = cmd.Process.Pid
	started := time.Now()
	go func() {
		err := cmd.Wait()
		winProc.Lock()
		winProc.pid = 0
		winProc.Unlock()
		if err == nil {
			return
		}
		var exit *exec.ExitError
		failed := errors.As(err, &exit) && exit.ExitCode() == exitNoWebView
		if failed || time.Since(started) < 20*time.Second {
			reason := windowError(logPath, err)
			slog.Error("встроенное окно не открылось", "reason", reason, "log", logPath)
			windowFailed(url, reason)
		}
	}()
	return nil
}

func windowLogPath() string {
	dir, err := config.StateDir()
	if err != nil {
		return ""
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	return filepath.Join(dir, "window.log")
}

var failShown atomic.Bool

func windowFailed(url, reason string) {
	if !failShown.CompareAndSwap(false, true) {
		return
	}
	defer failShown.Store(false)
	text := "Окно программы не открылось.\n\nПричина: " + reason + "\n\n"
	if strings.Contains(reason, "WebView2 Runtime") {
		text += "Установите WebView2 Runtime с сайта Microsoft\n(developer.microsoft.com/microsoft-edge/webview2)\nи откройте окно ещё раз.\n\n"
	}
	if p := windowLogPath(); p != "" {
		text += "Подробности — в файле " + p + "\n\n"
	}
	text += "Скробблер продолжает работать в трее. Открыть окно пока в браузере Microsoft Edge?"
	if messageBox(windowTitle, text, mbYesNo|mbIconWarning|mbSetForeground|mbTopmost) != idYes {
		return
	}
	if err := openAppWindow(url); err != nil {
		slog.Error("не удалось открыть окно в Edge", "err", err)
	}
}

func replaceRunning() func() {
	if err := requestStop(); err != nil {
		return nil
	}
	for i := 0; i < 120; i++ {
		time.Sleep(250 * time.Millisecond)
		if release, err := acquireInstance(); err == nil {
			return release
		}
	}
	return nil
}

func refreshAutostart(e *env) {
	cur := autostartValue()
	if cur == "" {
		return
	}
	want, err := autostartCommandLine()
	if err != nil || strings.EqualFold(cur, want) {
		return
	}
	if err := setAutostart(e, true); err != nil {
		e.log.Warn("не удалось обновить автозапуск", "err", err)
		return
	}
	e.log.Info("автозапуск переключён на эту копию программы", "was", cur, "now", want)
}

func windowCommand(url string) (*exec.Cmd, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(self, "window")
	cmd.Env = append(os.Environ(), envWindowURL+"="+url, envWindowParent+"="+strconv.Itoa(os.Getpid()))
	return cmd, nil
}

func showRunningWindow(url string) error {
	if raiseAppWindow(0) {
		return nil
	}
	allowForeground()
	if err := signalEvent(showEvent); err == nil {
		return nil
	}
	windowFailed(url, "работающая копия программы не ответила на просьбу открыть окно")
	return nil
}

func watchShow(ctx context.Context) <-chan struct{} {
	name, err := windows.UTF16PtrFromString(showEvent)
	if err != nil {
		return nil
	}
	h, err := windows.CreateEvent(nil, 0, 0, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil
	}
	ch := make(chan struct{}, 1)
	go func() {
		defer windows.CloseHandle(h)
		for ctx.Err() == nil {
			ev, err := windows.WaitForSingleObject(h, 500)
			if err == nil && ev == windows.WAIT_OBJECT_0 {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}()
	return ch
}
