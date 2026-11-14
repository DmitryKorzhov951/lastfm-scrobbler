package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
)

type env struct {
	cfg         config.Config
	cfgPath     string
	sessionPath string
	queuePath   string
	session     config.Session
	log         *slog.Logger
}

func main() {
	flags := flag.NewFlagSet("lastfm-scrobbler", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	trayOnly := flags.Bool("tray", false, "не открывать окно при запуске, ждать в трее")
	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	command, args := "app", flags.Args()
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch command {
	case "window":
		log := slog.New(slog.NewTextHandler(os.Stderr, nil)).With("proc", "window")
		slog.SetDefault(log)
		if err := cmdWindow(args); err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка:", err)
			os.Exit(exitNoWebView)
		}
	case "demo":
		if err := cmdDemo(ctx, args); err != nil && !isCanceled(err) {
			fatal(err)
		}
	case "app":
		if err := setupBackground(); err != nil {
			fatal(err)
		}
		e, err := newEnv()
		if err != nil {
			fatal(err)
		}
		if err := cmdApp(ctx, e, *trayOnly); err != nil && !isCanceled(err) {
			fatal(err)
		}
	default:
		fatal(fmt.Errorf("неизвестная команда %q", command))
	}
}

func isCanceled(err error) bool {
	return err == context.Canceled || strings.Contains(err.Error(), context.Canceled.Error())
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Ошибка:", err)
	os.Exit(1)
}

func newEnv() (*env, error) {
	cfgPath, err := config.Path()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	sessionPath, err := config.SessionPath()
	if err != nil {
		return nil, err
	}
	session, err := config.LoadSession(sessionPath)
	if err != nil {
		return nil, err
	}
	queuePath, err := config.QueuePath()
	if err != nil {
		return nil, err
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))
	slog.SetDefault(log)

	return &env{
		cfg:         cfg,
		cfgPath:     cfgPath,
		sessionPath: sessionPath,
		queuePath:   queuePath,
		session:     session,
		log:         log,
	}, nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
