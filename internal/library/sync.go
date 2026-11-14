package library

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

type Page struct {
	Plays      []Play
	TotalPages int
	Total      int
}

type Source interface {
	RecentTracks(ctx context.Context, user string, page int, from, to time.Time) (Page, error)
}

type Status struct {
	State    string     `json:"state"`
	Plays    int        `json:"plays"`
	Total    int        `json:"total"`
	Error    string     `json:"error,omitempty"`
	RetryAt  *time.Time `json:"retryAt,omitempty"`
	SyncedAt *time.Time `json:"syncedAt,omitempty"`
}

const (
	StateOff       = "off"
	StateImporting = "importing"
	StateReady     = "ready"
	StateError     = "error"
)

type Syncer struct {
	Store       *Store
	Log         *slog.Logger
	PagePause   time.Duration
	QuickEvery  time.Duration
	DeepEvery   time.Duration
	QuickMargin time.Duration
	DeepMargin  time.Duration
	Now         func() time.Time
	ErrorText   func(error) string

	wake chan struct{}

	op sync.Mutex

	mu       sync.Mutex
	src      Source
	user     string
	gen      int
	failures int
	errText  string
	retryAt  time.Time
	lastDeep time.Time
	lastSync time.Time
}

func NewSyncer(store *Store, log *slog.Logger) *Syncer {
	return &Syncer{
		Store:       store,
		Log:         log,
		PagePause:   250 * time.Millisecond,
		QuickEvery:  3 * time.Minute,
		DeepEvery:   6 * time.Hour,
		QuickMargin: time.Hour,
		DeepMargin:  15 * 24 * time.Hour,
		Now:         time.Now,
		ErrorText:   func(err error) string { return err.Error() },
		wake:        make(chan struct{}, 1),
	}
}

func (y *Syncer) SetAccount(src Source, user string) {
	y.mu.Lock()
	y.src, y.user = src, user
	y.gen++
	y.failures, y.errText, y.retryAt = 0, "", time.Time{}
	y.lastDeep, y.lastSync = time.Time{}, time.Time{}
	y.mu.Unlock()
	y.Kick()
}

func (y *Syncer) Reload() error {
	y.op.Lock()
	defer y.op.Unlock()
	y.mu.Lock()
	user := y.user
	y.gen++
	y.failures, y.errText, y.retryAt = 0, "", time.Time{}
	y.lastDeep, y.lastSync = time.Time{}, time.Time{}
	y.mu.Unlock()
	if err := y.Store.Reset(user); err != nil {
		return err
	}
	y.Kick()
	return nil
}

func (y *Syncer) SyncNow() {
	y.mu.Lock()
	fresh := !y.lastSync.IsZero() && y.Now().Sub(y.lastSync) < syncNowGap
	if !fresh {
		y.lastSync, y.retryAt = time.Time{}, time.Time{}
	}
	y.mu.Unlock()
	if !fresh {
		y.Kick()
	}
}

const syncNowGap = 10 * time.Second

func (y *Syncer) Kick() {
	select {
	case y.wake <- struct{}{}:
	default:
	}
}

func (y *Syncer) Status() Status {
	y.mu.Lock()
	src, errText, retryAt := y.src, y.errText, y.retryAt
	y.mu.Unlock()
	m := y.Store.Meta()
	st := Status{Plays: y.Store.Len(), Total: m.Total}
	switch {
	case src == nil:
		st.State = StateOff
	case errText != "":
		st.State = StateError
		st.Error = errText
		if !retryAt.IsZero() {
			st.RetryAt = &retryAt
		}
	case !m.Complete:
		st.State = StateImporting
	default:
		st.State = StateReady
	}
	if m.SyncedAt > 0 {
		t := time.Unix(m.SyncedAt, 0)
		st.SyncedAt = &t
	}
	return st
}

func (y *Syncer) account() (Source, string, int) {
	y.mu.Lock()
	defer y.mu.Unlock()
	return y.src, y.user, y.gen
}

func (y *Syncer) current(gen int) bool {
	y.mu.Lock()
	defer y.mu.Unlock()
	return y.gen == gen
}

func (y *Syncer) Run(ctx context.Context) {
	for {
		wait := y.step(ctx)
		if ctx.Err() != nil {
			return
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-y.wake:
			t.Stop()
		case <-t.C:
		}
	}
}

func (y *Syncer) step(ctx context.Context) time.Duration {
	src, user, gen := y.account()
	if src == nil || user == "" {
		return time.Hour
	}
	y.mu.Lock()
	retryAt := y.retryAt
	y.mu.Unlock()
	if now := y.Now(); now.Before(retryAt) {
		return retryAt.Sub(now)
	}

	if m := y.Store.Meta(); m.User != user {
		if m.User != "" {
			y.Log.Info("история прослушиваний другого аккаунта стёрта", "was", m.User, "now", user)
		}
		if err := y.Store.Reset(user); err != nil {
			return y.fail(gen, err)
		}
	}

	if !y.Store.Meta().Complete {
		if err := y.importAll(ctx, src, user, gen); err != nil {
			return y.fail(gen, err)
		}
	}

	now := y.Now()
	y.mu.Lock()
	deep := y.lastDeep.IsZero() || now.Sub(y.lastDeep) >= y.DeepEvery
	quick := y.lastSync.IsZero() || now.Sub(y.lastSync) >= y.QuickEvery
	y.mu.Unlock()
	if deep || quick {
		margin := y.QuickMargin
		if deep {
			margin = y.DeepMargin
		}
		if err := y.syncRecent(ctx, src, user, gen, margin); err != nil {
			return y.fail(gen, err)
		}
		y.mu.Lock()
		if y.gen == gen {
			y.lastSync = now
			if deep {
				y.lastDeep = now
			}
			y.failures, y.errText, y.retryAt = 0, "", time.Time{}
		}
		y.mu.Unlock()
	}
	return y.QuickEvery
}

func (y *Syncer) fail(gen int, err error) time.Duration {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errStale) {
		return time.Second
	}
	y.mu.Lock()
	defer y.mu.Unlock()
	if y.gen != gen {
		return time.Second
	}
	y.failures++
	delay := 30 * time.Second
	for i := 1; i < y.failures && delay < 30*time.Minute; i++ {
		delay *= 2
	}
	if delay > 30*time.Minute {
		delay = 30 * time.Minute
	}
	y.errText = y.ErrorText(err)
	y.retryAt = y.Now().Add(delay)
	y.Log.Warn("не удалось загрузить историю из Last.fm", "err", err, "retry_in", delay)
	return delay
}

type staleError struct{}

func (staleError) Error() string { return "аккаунт сменился" }

var errStale error = staleError{}

func (y *Syncer) pause(ctx context.Context) error {
	if y.PagePause <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(y.PagePause)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (y *Syncer) importAll(ctx context.Context, src Source, user string, gen int) error {
	m := y.Store.Meta()
	if m.ImportTo == 0 {
		m.ImportTo = y.Now().Unix()
		m.NextPage = 1
		if err := y.Store.SaveMeta(m); err != nil {
			return err
		}
		y.Log.Info("загрузка истории из Last.fm начата", "user", user)
	}
	if m.NextPage < 1 {
		m.NextPage = 1
	}
	for {
		p, err := src.RecentTracks(ctx, user, m.NextPage, time.Time{}, time.Unix(m.ImportTo, 0))
		if err != nil {
			return err
		}
		done, err := y.savePage(gen, &m, p)
		if err != nil {
			return err
		}
		y.mu.Lock()
		y.failures, y.errText, y.retryAt = 0, "", time.Time{}
		y.mu.Unlock()
		if done {
			y.Log.Info("история из Last.fm загружена", "plays", y.Store.Len())
			return nil
		}
		if err := y.pause(ctx); err != nil {
			return err
		}
	}
}

func (y *Syncer) savePage(gen int, m *Meta, p Page) (bool, error) {
	y.op.Lock()
	defer y.op.Unlock()
	if !y.current(gen) {
		return false, errStale
	}
	if _, err := y.Store.Add(p.Plays); err != nil {
		return false, err
	}
	m.TotalPages, m.Total = p.TotalPages, p.Total
	m.NextPage++
	done := m.NextPage > p.TotalPages
	if done {
		m.Complete = true
		m.SyncedAt = m.ImportTo
	}
	return done, y.Store.SaveMeta(*m)
}

func (y *Syncer) addRecent(gen int, plays []Play) (int, error) {
	y.op.Lock()
	defer y.op.Unlock()
	if !y.current(gen) {
		return 0, errStale
	}
	return y.Store.Add(plays)
}

func (y *Syncer) syncRecent(ctx context.Context, src Source, user string, gen int, margin time.Duration) error {
	m := y.Store.Meta()
	from := time.Unix(m.SyncedAt, 0).Add(-margin)
	started := y.Now()
	added := 0
	for page := 1; ; page++ {
		p, err := src.RecentTracks(ctx, user, page, from, time.Time{})
		if err != nil {
			return err
		}
		n, err := y.addRecent(gen, p.Plays)
		if err != nil {
			return err
		}
		added += n
		if page >= p.TotalPages {
			break
		}
		if err := y.pause(ctx); err != nil {
			return err
		}
	}
	if added > 0 {
		y.Log.Info("история дополнена из Last.fm", "added", added)
	}
	y.op.Lock()
	defer y.op.Unlock()
	if !y.current(gen) {
		return errStale
	}
	m = y.Store.Meta()
	m.SyncedAt = started.Unix()
	return y.Store.SaveMeta(m)
}
