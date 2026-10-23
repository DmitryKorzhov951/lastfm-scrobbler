//go:build windows

package gsmtc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/media/control"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/aimp"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

const asyncTimeout = 5 * time.Second

type Options struct {
	Poll  time.Duration
	Allow []string
	Deny  []string
	Log   *slog.Logger
	Now   func() time.Time
}

const maxPoll = 2 * time.Second

func (o *Options) normalize() {
	if o.Poll <= 0 || o.Poll > maxPoll {
		o.Poll = maxPoll
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
}

type Watcher struct {
	opts      Options
	events    chan media.State
	closed    chan struct{}
	closeOnce sync.Once
}

func NewWatcher(opts Options) (*Watcher, error) {
	opts.normalize()
	return &Watcher{
		opts:   opts,
		events: make(chan media.State, 64),
		closed: make(chan struct{}),
	}, nil
}

func (w *Watcher) Events() <-chan media.State { return w.events }

func (w *Watcher) Close() error {
	w.closeOnce.Do(func() { close(w.closed) })
	return nil
}

func (w *Watcher) Run(ctx context.Context) error {
	defer close(w.events)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	initWinRT(w.opts.Log)

	mgr, err := requestManager()
	if err != nil {
		return fmt.Errorf("системные медиаэлементы Windows недоступны: %w "+
			"(нужна Windows 10 версии 1809 или новее)", err)
	}
	defer mgr.Release()

	seen := make(map[string]bool)
	poll := func() {
		raws, err := readSessions(mgr)
		if err != nil {
			w.opts.Log.Debug("не удалось прочитать медиасессии", "err", err)
			return
		}
		now := w.opts.Now()
		current := make(map[string]bool, len(raws))
		for _, st := range withAIMP(statesOf(raws, now), now, w.opts.Log) {
			if !Allowed(st.Name, w.opts.Allow, w.opts.Deny) {
				continue
			}
			current[st.ID] = true
			if !seen[st.ID] {
				w.opts.Log.Info("найден плеер", "player", st.Name, "identity", st.Identity)
			}
			w.send(ctx, st)
		}
		for id := range seen {
			if !current[id] {
				w.opts.Log.Info("плеер закрыт", "player", AppName(id))
				w.send(ctx, media.State{ID: id, Name: AppName(id), Gone: true, At: now})
			}
		}
		seen = current
	}

	poll()
	ticker := time.NewTicker(w.opts.Poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.closed:
			return nil
		case <-ticker.C:
			poll()
		}
	}
}

func (w *Watcher) send(ctx context.Context, st media.State) {
	select {
	case w.events <- st:
	case <-ctx.Done():
	case <-w.closed:
	}
}

func ListPlayers(ctx context.Context) ([]media.State, error) {
	type result struct {
		states []media.State
		err    error
	}
	done := make(chan result, 1)

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		initWinRT(slog.Default())

		mgr, err := requestManager()
		if err != nil {
			done <- result{err: fmt.Errorf("системные медиаэлементы Windows недоступны: %w", err)}
			return
		}
		defer mgr.Release()

		raws, err := readSessions(mgr)
		if err != nil {
			done <- result{err: err}
			return
		}
		now := time.Now()
		done <- result{states: withAIMP(statesOf(raws, now), now, slog.Default())}
	}()

	select {
	case r := <-done:
		return r.states, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func statesOf(raws []Raw, now time.Time) []media.State {
	states := make([]media.State, 0, len(raws)+1)
	for _, r := range Pick(raws) {
		states = append(states, ToState(r, now))
	}
	return states
}

func withAIMP(states []media.State, now time.Time, log *slog.Logger) []media.State {
	for _, st := range states {
		if st.Name == "aimp" {
			return states
		}
	}
	st, ok, err := aimp.Read(now)
	if err != nil {
		log.Debug("не удалось прочитать AIMP", "err", err)
		return states
	}
	if ok {
		states = append(states, st)
	}
	return states
}

func initWinRT(log *slog.Logger) {
	const sFalse = 1
	if err := ole.RoInitialize(1); err != nil {
		var oleErr *ole.OleError
		if errors.As(err, &oleErr) && oleErr.Code() == sFalse {
			return
		}
		log.Debug("RoInitialize", "err", err)
	}
}

func requestManager() (*control.GlobalSystemMediaTransportControlsSessionManager, error) {
	op, err := control.GlobalSystemMediaTransportControlsSessionManagerRequestAsync()
	if err != nil {
		return nil, err
	}
	defer op.Release()

	ptr, err := await(op, control.SignatureGlobalSystemMediaTransportControlsSessionManager)
	if err != nil {
		return nil, err
	}
	if ptr == nil {
		return nil, errors.New("Windows не вернула менеджер медиасессий")
	}
	return (*control.GlobalSystemMediaTransportControlsSessionManager)(ptr), nil
}

func readSessions(mgr *control.GlobalSystemMediaTransportControlsSessionManager) ([]Raw, error) {
	vec, err := mgr.GetSessions()
	if err != nil {
		return nil, err
	}
	defer vec.Release()

	n, err := vec.GetSize()
	if err != nil {
		return nil, err
	}

	out := make([]Raw, 0, n)
	for i := uint32(0); i < n; i++ {
		ptr, err := vec.GetAt(i)
		if err != nil || ptr == nil {
			continue
		}
		s := (*control.GlobalSystemMediaTransportControlsSession)(ptr)
		r, err := readSession(s)
		s.Release()
		if err != nil {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func readSession(s *control.GlobalSystemMediaTransportControlsSession) (Raw, error) {
	var r Raw

	id, err := s.GetSourceAppUserModelId()
	if err != nil {
		return r, err
	}
	r.AUMID = id

	if pi, err := s.GetPlaybackInfo(); err == nil && pi != nil {
		if st, err := pi.GetPlaybackStatus(); err == nil {
			r.Status = int32(st)
		}
		pi.Release()
	}

	if tl, err := s.GetTimelineProperties(); err == nil && tl != nil {
		if v, err := tl.GetStartTime(); err == nil {
			r.Start = v.Duration
		}
		if v, err := tl.GetEndTime(); err == nil {
			r.End = v.Duration
		}
		if v, err := tl.GetPosition(); err == nil {
			r.Position = v.Duration
		}
		if v, err := tl.GetLastUpdatedTime(); err == nil {
			r.Updated = v.UniversalTime
		}
		tl.Release()
	}

	op, err := s.TryGetMediaPropertiesAsync()
	if err != nil {
		return r, err
	}
	ptr, err := await(op, control.SignatureGlobalSystemMediaTransportControlsSessionMediaProperties)
	op.Release()
	if err != nil {
		return r, err
	}
	if ptr == nil {
		return r, nil
	}

	mp := (*control.GlobalSystemMediaTransportControlsSessionMediaProperties)(ptr)
	defer mp.Release()
	r.Title, _ = mp.GetTitle()
	r.Artist, _ = mp.GetArtist()
	r.Album, _ = mp.GetAlbumTitle()
	r.AlbumArtist, _ = mp.GetAlbumArtist()
	r.TrackNumber, _ = mp.GetTrackNumber()
	return r, nil
}

func await(op *foundation.IAsyncOperation, signature string) (unsafe.Pointer, error) {
	iid := winrt.ParameterizedInstanceGUID(foundation.GUIDAsyncOperationCompletedHandler, signature)

	done := make(chan foundation.AsyncStatus, 1)
	handler := foundation.NewAsyncOperationCompletedHandler(ole.NewGUID(iid),
		func(_ *foundation.AsyncOperationCompletedHandler, _ *foundation.IAsyncOperation, status foundation.AsyncStatus) {
			select {
			case done <- status:
			default:
			}
		})
	defer handler.Release()

	if err := op.SetCompleted(handler); err != nil {
		return nil, err
	}

	select {
	case status := <-done:
		if status != foundation.AsyncStatusCompleted {
			return nil, fmt.Errorf("асинхронная операция Windows завершилась со статусом %d", status)
		}
		return op.GetResults()
	case <-time.After(asyncTimeout):
		return nil, errors.New("Windows не ответила на запрос вовремя")
	}
}
