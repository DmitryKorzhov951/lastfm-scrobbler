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
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
)

const platformName = "Windows"

const platformCommands = `  start     запустить программу в фоне: значок в трее, окно — по щелчку
  stop      остановить работающую программу (очередь будет дослана)
  autostart автозапуск при входе в Windows: on | off | status
  run       работать в этой консоли без окна, выход — Ctrl+C
`

const platformExamples = `  lastfm-scrobbler start                запустить в фоне
  lastfm-scrobbler autostart on         запускать при входе в Windows
`

const (
	instanceMutex = `Local\lastfm-scrobbler-instance`
	stopEvent     = `Local\lastfm-scrobbler-stop`
	showEvent     = `Local\lastfm-scrobbler-show`

	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "lastfm-scrobbler"

	backgroundExe = "lastfm-scrobblerw.exe"

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
		return nil, errors.New("скробблер уже запущен (остановить: lastfm-scrobbler stop)")
	}
	if err != nil {
		return nil, fmt.Errorf("создание мьютекса экземпляра: %w", err)
	}
	return func() { windows.CloseHandle(h) }, nil
}

func isRunning() bool {
	name, err := windows.UTF16PtrFromString(instanceMutex)
	if err != nil {
		return false
	}
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
	if err != nil {
		return false
	}
	windows.CloseHandle(h)
	return true
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

func selfDir() (string, string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	return self, filepath.Dir(self), nil
}

func customConfigArgs(e *env) []string {
	def, err := config.Path()
	if err == nil && strings.EqualFold(filepath.Clean(def), filepath.Clean(e.cfgPath)) {
		return nil
	}
	return []string{"-config", e.cfgPath}
}

func backgroundCommand(e *env) (*exec.Cmd, error) {
	self, dir, err := selfDir()
	if err != nil {
		return nil, err
	}
	var cmd *exec.Cmd
	if w := filepath.Join(dir, backgroundExe); fileExists(w) {
		cmd = exec.Command(w, append(customConfigArgs(e), "-tray")...)
	} else {
		args := append(customConfigArgs(e), "-background", "-tray", "app")
		cmd = exec.Command(self, args...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
	}
	return cmd, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func cmdStart(e *env, _ []string) error {
	if isRunning() {
		fmt.Println("Скробблер уже работает в фоне.")
		return nil
	}
	cmd, err := backgroundCommand(e)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("запуск фонового процесса: %w", err)
	}
	_ = cmd.Process.Release()

	for i := 0; i < 40; i++ {
		if isRunning() {
			fmt.Println("Скробблер запущен: значок в трее, окно — по щелчку по значку.")
			fmt.Println("Журнал:     ", logPath())
			fmt.Println("Остановить:  lastfm-scrobbler stop")
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("фоновый процесс не запустился — подробности в журнале: %s", logPath())
}

func cmdStop(_ *env, _ []string) error {
	if !isRunning() {
		fmt.Println("Фоновый скробблер не запущен.")
		return nil
	}
	if err := requestStop(); err != nil {
		return fmt.Errorf("не удалось подать сигнал остановки: %w", err)
	}
	fmt.Print("Останавливаю (досылаю очередь)...")
	for i := 0; i < 100; i++ {
		if !isRunning() {
			fmt.Println(" готово.")
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Println()
	return errors.New("скробблер не остановился за 20 секунд — завершите процесс lastfm-scrobbler в диспетчере задач; " +
		"неотправленные прослушивания сохранены и уйдут при следующем запуске")
}

func autostartCommandLine(e *env) (cmdline string, flashes bool, err error) {
	self, dir, err := selfDir()
	if err != nil {
		return "", false, err
	}
	quote := func(s string) string { return `"` + s + `"` }
	extra := customConfigArgs(e)
	for i := range extra {
		extra[i] = quote(extra[i])
	}

	if w := filepath.Join(dir, backgroundExe); fileExists(w) {
		parts := append([]string{quote(w)}, extra...)
		return strings.Join(append(parts, "-tray"), " "), false, nil
	}
	parts := append([]string{quote(self)}, extra...)
	parts = append(parts, "-background", "-tray", "app")
	return strings.Join(parts, " "), true, nil
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
	cmdline, _, err := autostartCommandLine(e)
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

func cmdAutostart(e *env, args []string) error {
	action := "status"
	if len(args) > 0 {
		action = args[0]
	}

	switch action {
	case "on":
		cmdline, flashes, err := autostartCommandLine(e)
		if err != nil {
			return err
		}
		if err := setAutostart(e, true); err != nil {
			return err
		}
		fmt.Println("Автозапуск включён:", cmdline)
		if flashes {
			fmt.Printf("Рядом нет %s — при входе на мгновение мелькнёт окно консоли.\n", backgroundExe)
			fmt.Println("Чтобы этого не было, положите его в ту же папку и выполните autostart on ещё раз.")
		}
		return nil

	case "off":
		if err := setAutostart(e, false); err != nil {
			return err
		}
		fmt.Println("Автозапуск выключен.")
		return nil

	case "status":
		if v := autostartValue(); v != "" {
			fmt.Println("Автозапуск включён:", v)
		} else {
			fmt.Println("Автозапуск выключен.")
		}
		return nil

	default:
		return fmt.Errorf("неизвестное действие %q (доступны: on, off, status)", action)
	}
}

func platformStatus() [][2]string {
	running := "не запущен (запустить: lastfm-scrobbler start)"
	if isRunning() {
		running = "работает"
	}
	auto := "выключен"
	if autostartValue() != "" {
		auto = "включён"
	}
	return [][2]string{
		{"Фоновый режим", running},
		{"Автозапуск", auto},
		{"Журнал", logPath()},
	}
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
		return openAppWindow(url)
	}
	allowForeground()
	if err := cmd.Start(); err != nil {
		slog.Warn("не удалось запустить окно, открываю его в Edge", "err", err)
		return openAppWindow(url)
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
		noWebView := errors.As(err, &exit) && exit.ExitCode() == exitNoWebView
		if noWebView || time.Since(started) < 20*time.Second {
			slog.Warn("встроенное окно не открылось, открываю его в Edge", "err", err)
			if err := openAppWindow(url); err != nil {
				slog.Error("не удалось открыть окно", "err", err)
			}
		}
	}()
	return nil
}

func windowCommand(url string) (*exec.Cmd, error) {
	self, dir, err := selfDir()
	if err != nil {
		return nil, err
	}
	var cmd *exec.Cmd
	switch w := filepath.Join(dir, backgroundExe); {
	case backgroundBuild == "1":
		cmd = exec.Command(self, "window")
	case fileExists(w):
		cmd = exec.Command(w, "window")
	default:
		cmd = exec.Command(self, "-background", "window")
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	}
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
	return openAppWindow(url)
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
