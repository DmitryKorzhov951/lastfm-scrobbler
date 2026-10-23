package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/history"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lastfm"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/player"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/queue"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/radio"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/scrobbler"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/sdnotify"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/version"
)

type service struct {
	log         *slog.Logger
	dryRun      bool
	cfgPath     string
	sessionPath string

	queue  *queue.Queue
	hist   *history.Store
	sub    *submitter
	radio  *radio.Radio
	paused atomic.Bool
	calls  chan func(*scrobbler.Engine)

	newWatcher func(player.Options) (player.Watcher, error)

	mu      sync.Mutex
	cfg     config.Config
	session config.Session
	snaps   []scrobbler.Snapshot
	login   *pendingLogin
}

type pendingLogin struct {
	client  *lastfm.Client
	token   string
	url     string
	started time.Time
}

func newService(e *env, dryRun bool) (*service, error) {
	q, err := queue.Open(e.queuePath, e.cfg.QueueLimit)
	if err != nil {
		if q == nil {
			return nil, err
		}
		e.log.Warn("очередь скробблов сброшена", "err", err)
	}
	histPath, err := config.HistoryPath()
	if err != nil {
		return nil, err
	}
	hist, err := history.Open(histPath, 0)
	if err != nil {
		if hist == nil {
			return nil, err
		}
		e.log.Warn("история прослушиваний сброшена", "err", err)
	}
	q.SetLastID(hist.LastID())

	radioPath, err := config.RadioPath()
	if err != nil {
		return nil, err
	}
	rad, err := radio.Open(radioPath, e.log, version.UserAgent())
	if err != nil {
		if rad == nil {
			return nil, err
		}
		e.log.Warn("список радиостанций сброшен", "err", err)
	}

	s := &service{
		log:         e.log,
		dryRun:      dryRun,
		cfgPath:     e.cfgPath,
		sessionPath: e.sessionPath,
		queue:       q,
		hist:        hist,
		radio:       rad,
		calls:       make(chan func(*scrobbler.Engine)),
		newWatcher:  player.New,
		cfg:         e.cfg,
		session:     e.session,
	}
	s.sub = newSubmitter(s.newClient(e.cfg, e.session.Key), q, hist, e.cfg.RetryInterval.Duration(), dryRun, e.log)
	return s, nil
}

func (s *service) newClient(cfg config.Config, sessionKey string) *lastfm.Client {
	if cfg.APIKey == "" || cfg.APISecret == "" {
		return nil
	}
	c := lastfm.New(cfg.APIKey, cfg.APISecret)
	c.UserAgent = version.UserAgent()
	c.SessionKey = sessionKey
	if v := os.Getenv("LASTFM_API_ENDPOINT"); v != "" {
		c.Endpoint = v
	}
	return c
}

func engineOptions(cfg config.Config) scrobbler.Options {
	return scrobbler.Options{
		MinTrackLength:        cfg.MinTrackLength.Duration(),
		MaxPlayTime:           cfg.MaxPlayTime.Duration(),
		Percent:               cfg.ScrobblePercent,
		SkipUnknownLength:     cfg.SkipUnknownLength,
		IgnoreInitialPosition: !player.PositionReliable,
	}
}

type pausableSink struct {
	inner  scrobbler.Sink
	paused *atomic.Bool
}

func (p pausableSink) NowPlaying(t lastfm.Track) {
	if !p.paused.Load() {
		p.inner.NowPlaying(t)
	}
}

func (p pausableSink) Scrobble(sc lastfm.Scrobble) {
	if !p.paused.Load() {
		p.inner.Scrobble(sc)
	}
}

func (s *service) Run(ctx context.Context, ready func()) error {
	cfg := s.config()
	watcher, err := s.newWatcher(player.Options{Poll: cfg.PollInterval.Duration(), Log: s.log})
	if err != nil {
		return err
	}
	defer watcher.Close()

	engine := scrobbler.New(engineOptions(cfg), pausableSink{inner: s.sub, paused: &s.paused}, s.log)

	watchCtx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()

	subCtx, stopSub := context.WithCancel(context.Background())
	subDone := make(chan struct{})
	go func() {
		defer close(subDone)
		s.sub.Run(subCtx)
	}()

	watchErr := make(chan error, 1)
	go func() { watchErr <- watcher.Run(watchCtx) }()
	go s.radio.Run(watchCtx)

	s.log.Info("скробблер запущен",
		"version", version.Version,
		"user", s.account().Username,
		"queue", s.queue.Len(),
		"source", player.Source,
		"dry_run", s.dryRun)
	if ready != nil {
		ready()
	}

	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	watchdog := sdnotify.WatchdogInterval()
	var watchdogC <-chan time.Time
	if watchdog > 0 {
		wt := time.NewTicker(watchdog)
		defer wt.Stop()
		watchdogC = wt.C
	}

	statusTick := time.NewTicker(30 * time.Second)
	defer statusTick.Stop()

	events := watcher.Events()
	var runErr error

loop:
	for {
		select {
		case <-ctx.Done():
			break loop

		case err := <-watchErr:
			if err != nil && !isCanceled(err) {
				runErr = fmt.Errorf("слежение за плеерами прервано: %w", err)
			}
			break loop

		case st, ok := <-events:
			if !ok {
				break loop
			}
			s.update(engine, st)

		case st := <-s.radio.Events():
			s.update(engine, st)

		case now := <-tick.C:
			engine.Tick(now)
			s.publish(engine)

		case f := <-s.calls:
			f(engine)
			s.publish(engine)

		case <-statusTick.C:
			sdnotify.Status("%s; в очереди %d", currentTrackLine(engine), s.queue.Len())

		case <-watchdogC:
			sdnotify.Watchdog()
		}
	}

	sdnotify.Stopping()
	sdnotify.Status("завершение работы")
	s.log.Info("остановка скробблера")

	stopWatch()
	stopSub()
	<-subDone
	return runErr
}

func (s *service) update(engine *scrobbler.Engine, st media.State) {
	if !st.Gone {
		if err := s.hist.SeePlayer(st.Name, st.Identity); err != nil {
			s.log.Warn("не удалось запомнить плеер", "err", err)
		}
		if !s.playerAllowed(st.Name) {
			st = media.State{ID: st.ID, Name: st.Name, At: st.At, Gone: true}
		}
	}
	engine.Update(st)
	s.publish(engine)
}

func (s *service) publish(engine *scrobbler.Engine) {
	snaps := engine.Snapshots()
	s.mu.Lock()
	s.snaps = snaps
	s.mu.Unlock()
}

func (s *service) do(ctx context.Context, f func(*scrobbler.Engine)) {
	done := make(chan struct{})
	select {
	case s.calls <- func(e *scrobbler.Engine) { f(e); close(done) }:
		select {
		case <-done:
		case <-ctx.Done():
		}
	case <-ctx.Done():
	}
}

func (s *service) config() config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

func (s *service) account() config.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session
}

func (s *service) playerAllowed(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return playerAllowed(name, s.cfg.AllowedPlayers, s.cfg.IgnoredPlayers)
}

func playerAllowed(name string, allow, deny []string) bool {
	for _, d := range deny {
		if strings.EqualFold(strings.TrimSpace(d), name) {
			return false
		}
	}
	if len(allow) == 0 {
		return true
	}
	for _, a := range allow {
		if strings.EqualFold(strings.TrimSpace(a), name) {
			return true
		}
	}
	return false
}

func (s *service) activeSnapshot() (scrobbler.Snapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var best scrobbler.Snapshot
	found := false
	for _, sn := range s.snaps {
		if sn.Active {
			return sn, true
		}
		if !found && sn.Track.Valid() && sn.Status != media.Stopped {
			best, found = sn, true
		}
	}
	return best, found
}

func (s *service) SetPaused(p bool) {
	if s.paused.Swap(p) != p {
		s.log.Info("скробблинг " + map[bool]string{true: "приостановлен", false: "возобновлён"}[p])
	}
}

func (s *service) Paused() bool { return s.paused.Load() }

func (s *service) DeleteQueued(ids ...int64) error {
	if _, err := s.queue.Remove(ids...); err != nil {
		return err
	}
	return s.hist.Delete(ids...)
}

func (s *service) ClearQueue() error {
	return s.DeleteQueued(itemIDs(s.queue.Items())...)
}

func (s *service) ApplyConfig(ctx context.Context, cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := config.Save(s.cfgPath, cfg); err != nil {
		return fmt.Errorf("сохранение настроек: %w", err)
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	opts := engineOptions(cfg)
	s.do(ctx, func(e *scrobbler.Engine) { e.SetOptions(opts) })
	return nil
}

var apiKeyRe = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

var errBadKey = errors.New("ключ должен состоять из 32 символов 0–9 и a–f — скопируйте его со страницы Last.fm целиком")

func (s *service) SaveKeys(key, secret string) error {
	key, secret = strings.TrimSpace(key), strings.TrimSpace(secret)
	if !apiKeyRe.MatchString(key) {
		return fmt.Errorf("API key: %w", errBadKey)
	}
	if !apiKeyRe.MatchString(secret) {
		return fmt.Errorf("Shared secret: %w", errBadKey)
	}
	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()
	cfg.APIKey, cfg.APISecret = key, secret
	if err := config.Save(s.cfgPath, cfg); err != nil {
		return fmt.Errorf("сохранение настроек: %w", err)
	}
	s.mu.Lock()
	s.cfg = cfg
	s.login = nil
	s.mu.Unlock()
	return nil
}

func (s *service) StartLogin(ctx context.Context) (string, error) {
	cfg := s.config()
	client := s.newClient(cfg, "")
	if client == nil {
		return "", errors.New("сначала введите ключи приложения Last.fm")
	}
	token, err := client.GetToken(ctx)
	if err != nil {
		return "", loginError(err)
	}
	l := &pendingLogin{client: client, token: token, url: client.AuthURL(token), started: time.Now()}
	s.mu.Lock()
	s.login = l
	s.mu.Unlock()
	return l.url, nil
}

var errLoginPending = errors.New("ожидание подтверждения в браузере")

func (s *service) CheckLogin(ctx context.Context) (config.Session, error) {
	s.mu.Lock()
	l := s.login
	s.mu.Unlock()
	if l == nil {
		return config.Session{}, errors.New("вход не начат")
	}
	sess, err := l.client.GetSession(ctx, l.token)
	if err != nil {
		if apiErr, ok := lastfm.AsError(err); ok && apiErr.Code == lastfm.ErrCodeTokenNotAuth {
			return config.Session{}, errLoginPending
		}
		s.mu.Lock()
		s.login = nil
		s.mu.Unlock()
		return config.Session{}, loginError(err)
	}
	saved := config.Session{Username: sess.Name, Key: sess.Key, CreatedAt: time.Now().UTC()}
	if err := config.SaveSession(s.sessionPath, saved); err != nil {
		return config.Session{}, fmt.Errorf("сохранение сессии: %w", err)
	}
	s.mu.Lock()
	s.session = saved
	s.login = nil
	cfg := s.cfg
	s.mu.Unlock()
	s.sub.SetClient(s.newClient(cfg, saved.Key))
	s.log.Info("вход выполнен", "user", saved.Username)
	return saved, nil
}

func (s *service) Logout() error {
	if err := config.DeleteSession(s.sessionPath); err != nil {
		return fmt.Errorf("удаление сессии: %w", err)
	}
	s.mu.Lock()
	s.session = config.Session{}
	s.login = nil
	s.mu.Unlock()
	s.sub.SetClient(nil)
	s.log.Info("выполнен выход из аккаунта")
	return nil
}

func loginError(err error) error {
	apiErr, ok := lastfm.AsError(err)
	if !ok {
		return fmt.Errorf("нет связи с Last.fm: %w", err)
	}
	switch apiErr.Code {
	case lastfm.ErrCodeInvalidAPIKey:
		return errors.New("Last.fm не узнал API key — проверьте ключ")
	case lastfm.ErrCodeInvalidSig:
		return errors.New("Last.fm не принял подпись — проверьте Shared secret")
	case lastfm.ErrCodeTokenExpired:
		return errors.New("время на подтверждение истекло — начните вход заново")
	}
	return err
}
