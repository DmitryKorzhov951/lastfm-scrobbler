package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lastfm"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/version"
)

var backgroundBuild = ""

func usageText() string {
	return "lastfm-scrobbler — скробблер Last.fm для " + platformName + `

Использование:
  lastfm-scrobbler [флаги] <команда>

Команды:
  app       окно программы и значок в трее (так работает lastfm-scrobblerw.exe)
  open      открыть окно работающей программы или запустить её
  auth      авторизоваться в Last.fm и сохранить ключ сессии
` + platformCommands + `  status    показать настройки, очередь и найденные плееры
  queue     работа с очередью: list | flush | clear
  config    работа с настройками: path | init | show
  demo      показать окно на примерных данных, без Last.fm и плееров
  version   показать версию
  help      эта справка

Глобальные флаги:
  -config <путь>   файл настроек (где он по умолчанию — lastfm-scrobbler config path)
  -tray            команда app: не открывать окно при запуске, ждать в трее
  -v               подробный вывод (уровень логов debug)

Примеры:
  lastfm-scrobbler config init          записать ключи API
  lastfm-scrobbler auth                 привязать аккаунт Last.fm
` + platformExamples
}

type env struct {
	cfg         config.Config
	cfgPath     string
	sessionPath string
	queuePath   string
	session     config.Session
	log         *slog.Logger
}

func main() {
	var (
		cfgPath    string
		verbose    bool
		background bool
		trayOnly   bool
	)
	flags := flag.NewFlagSet("lastfm-scrobbler", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.Usage = func() { fmt.Fprint(os.Stderr, usageText()) }
	flags.StringVar(&cfgPath, "config", "", "путь к файлу настроек")
	flags.BoolVar(&verbose, "v", false, "подробный вывод")
	flags.BoolVar(&background, "background", false, "фоновый режим: без консоли, журнал в файл (Windows)")
	flags.BoolVar(&trayOnly, "tray", false, "не открывать окно при запуске, ждать в трее")

	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	args := flags.Args()
	command := "run"
	if len(args) > 0 {
		command = args[0]
		args = args[1:]
	}

	if backgroundBuild == "1" {
		background = true
		if command != "window" {
			command, args = "app", nil
		}
	}
	if background {
		if command != "run" && command != "app" && command != "window" {
			fatal(fmt.Errorf("флаг -background применим только к командам run и app"))
		}
		if err := setupBackground(); err != nil {
			fatal(err)
		}
	}

	if command == "help" || command == "-h" || command == "--help" {
		fmt.Print(usageText())
		return
	}
	if command == "version" {
		fmt.Printf("lastfm-scrobbler %s\n", version.Version)
		return
	}

	if command == "window" {
		log := slog.New(slog.NewTextHandler(os.Stderr, nil)).With("proc", "window")
		slog.SetDefault(log)
		if err := cmdWindow(args); err != nil {
			if errors.Is(err, errNoWebView) || errors.Is(err, errPageTimeout) || errors.Is(err, errNoLoader) {
				fmt.Fprintln(os.Stderr, "Ошибка:", err)
				os.Exit(exitNoWebView)
			}
			fatal(err)
		}
		return
	}

	if command == "demo" {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		if err := cmdDemo(ctx, args); err != nil && !isCanceled(err) {
			fatal(err)
		}
		return
	}

	e, err := newEnv(cfgPath, verbose)
	if err != nil {
		fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch command {
	case "app":
		err = cmdApp(ctx, e, args, trayOnly)
	case "open":
		err = cmdOpen(ctx, e, args)
	case "run":
		err = cmdRun(ctx, e, args)
	case "auth":
		err = cmdAuth(ctx, e, args)
	case "status":
		err = cmdStatus(ctx, e, args)
	case "queue":
		err = cmdQueue(ctx, e, args)
	case "config":
		err = cmdConfig(e, args)
	case "start":
		err = cmdStart(e, args)
	case "stop":
		err = cmdStop(e, args)
	case "autostart":
		err = cmdAutostart(e, args)
	default:
		fmt.Fprintf(os.Stderr, "неизвестная команда %q\n\n", command)
		fmt.Fprint(os.Stderr, usageText())
		os.Exit(2)
	}

	if err != nil && !isCanceled(err) {
		fatal(err)
	}
}

func isCanceled(err error) bool {
	return err == context.Canceled || strings.Contains(err.Error(), context.Canceled.Error())
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Ошибка:", err)
	os.Exit(1)
}

func newEnv(cfgPath string, verbose bool) (*env, error) {
	var err error
	if cfgPath == "" {
		if cfgPath, err = config.Path(); err != nil {
			return nil, err
		}
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

	level := parseLevel(cfg.LogLevel)
	if verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
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

func (e *env) client() (*lastfm.Client, error) {
	if e.cfg.APIKey == "" || e.cfg.APISecret == "" {
		return nil, fmt.Errorf("не заданы api_key/api_secret.\n"+
			"  1. Создайте приложение: https://www.last.fm/api/account/create\n"+
			"  2. Запишите ключи: lastfm-scrobbler config init\n"+
			"     (или задайте LASTFM_API_KEY и LASTFM_API_SECRET)\n"+
			"  Файл настроек: %s", e.cfgPath)
	}
	c := lastfm.New(e.cfg.APIKey, e.cfg.APISecret)
	c.UserAgent = version.UserAgent()
	c.SessionKey = e.session.Key
	if v := os.Getenv("LASTFM_API_ENDPOINT"); v != "" {
		c.Endpoint = v
		e.log.Warn("используется нестандартный адрес API Last.fm", "endpoint", v)
	}
	return c, nil
}
