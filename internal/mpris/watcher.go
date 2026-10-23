//go:build !windows

package mpris

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

type Options struct {
	Poll  time.Duration
	Allow []string
	Deny  []string
	Log   *slog.Logger
	Now   func() time.Time
}

func (o *Options) normalize() {
	if o.Poll <= 0 {
		o.Poll = 5 * time.Second
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
}

type Watcher struct {
	opts   Options
	conn   *dbus.Conn
	events chan media.State

	mu        sync.Mutex
	owners    map[string]string
	buses     map[string]string
	identity  map[string]string
	closeOnce sync.Once
}

func NewWatcher(opts Options) (*Watcher, error) {
	opts.normalize()

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("подключение к сессионной шине D-Bus: %w "+
			"(проверьте DBUS_SESSION_BUS_ADDRESS — сервис должен работать внутри графической сессии)", err)
	}

	w := &Watcher{
		opts:     opts,
		conn:     conn,
		events:   make(chan media.State, 64),
		owners:   make(map[string]string),
		buses:    make(map[string]string),
		identity: make(map[string]string),
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface(PropsIface),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchObjectPath(ObjectPath),
	); err != nil {
		conn.Close()
		return nil, fmt.Errorf("подписка на PropertiesChanged: %w", err)
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchArg0Namespace(strings.TrimSuffix(BusPrefix, ".")),
	); err != nil {
		conn.Close()
		return nil, fmt.Errorf("подписка на NameOwnerChanged: %w", err)
	}

	return w, nil
}

func (w *Watcher) Events() <-chan media.State { return w.events }

func (w *Watcher) Close() error {
	var err error
	w.closeOnce.Do(func() {
		err = w.conn.Close()
	})
	return err
}

func (w *Watcher) Run(ctx context.Context) error {
	defer close(w.events)

	signals := make(chan *dbus.Signal, 64)
	w.conn.Signal(signals)
	defer w.conn.RemoveSignal(signals)

	w.scan(ctx)

	ticker := time.NewTicker(w.opts.Poll)
	defer ticker.Stop()

	rescan := time.NewTicker(time.Minute)
	defer rescan.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case sig, ok := <-signals:
			if !ok {
				return fmt.Errorf("соединение с D-Bus закрыто")
			}
			w.handleSignal(ctx, sig)

		case <-ticker.C:
			for _, bus := range w.knownBuses() {
				w.emitState(ctx, bus)
			}

		case <-rescan.C:
			w.scan(ctx)
		}
	}
}

func (w *Watcher) allowed(bus string) bool {
	base := strings.ToLower(BaseName(bus))
	for _, d := range w.opts.Deny {
		if strings.EqualFold(strings.TrimSpace(d), base) {
			return false
		}
	}
	if len(w.opts.Allow) == 0 {
		return true
	}
	for _, a := range w.opts.Allow {
		if strings.EqualFold(strings.TrimSpace(a), base) {
			return true
		}
	}
	return false
}

func (w *Watcher) scan(ctx context.Context) {
	var names []string
	err := w.conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.ListNames", 0).Store(&names)
	if err != nil {
		w.opts.Log.Warn("не удалось получить список имён D-Bus", "err", err)
		return
	}
	seen := make(map[string]bool)
	for _, name := range names {
		if !strings.HasPrefix(name, BusPrefix) || !w.allowed(name) {
			continue
		}
		seen[name] = true
		if w.register(ctx, name) {
			w.opts.Log.Info("найден плеер", "player", ShortName(name), "identity", w.identityOf(ctx, name))
		}
		w.emitState(ctx, name)
	}
	for _, bus := range w.knownBuses() {
		if !seen[bus] {
			w.unregister(bus)
			w.send(ctx, media.State{ID: bus, Name: ShortName(bus), Gone: true, At: w.opts.Now()})
		}
	}
}

func (w *Watcher) handleSignal(ctx context.Context, sig *dbus.Signal) {
	switch sig.Name {
	case "org.freedesktop.DBus.NameOwnerChanged":
		if len(sig.Body) < 3 {
			return
		}
		name, _ := sig.Body[0].(string)
		newOwner, _ := sig.Body[2].(string)
		if !strings.HasPrefix(name, BusPrefix) || !w.allowed(name) {
			return
		}
		if newOwner == "" {
			w.unregister(name)
			w.opts.Log.Info("плеер закрыт", "player", ShortName(name))
			w.send(ctx, media.State{ID: name, Name: ShortName(name), Gone: true, At: w.opts.Now()})
			return
		}
		w.register(ctx, name)
		w.opts.Log.Info("найден плеер", "player", ShortName(name), "identity", w.identityOf(ctx, name))
		w.emitState(ctx, name)

	case PropsIface + ".PropertiesChanged":
		if len(sig.Body) < 1 {
			return
		}
		if iface, _ := sig.Body[0].(string); iface != PlayerIface {
			return
		}
		bus := w.busFor(string(sig.Sender))
		if bus == "" {
			w.scan(ctx)
			if bus = w.busFor(string(sig.Sender)); bus == "" {
				return
			}
		}
		w.emitState(ctx, bus)
	}
}

func (w *Watcher) register(ctx context.Context, bus string) bool {
	var owner string
	err := w.conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetNameOwner", 0, bus).Store(&owner)
	if err != nil || owner == "" {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if prev, ok := w.buses[bus]; ok && prev == owner {
		return false
	}
	w.buses[bus] = owner
	w.owners[owner] = bus
	return true
}

func (w *Watcher) unregister(bus string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if owner, ok := w.buses[bus]; ok {
		delete(w.owners, owner)
	}
	delete(w.buses, bus)
	delete(w.identity, bus)
}

func (w *Watcher) busFor(sender string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.owners[sender]
}

func (w *Watcher) knownBuses() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, 0, len(w.buses))
	for bus := range w.buses {
		out = append(out, bus)
	}
	return out
}

func (w *Watcher) identityOf(ctx context.Context, bus string) string {
	w.mu.Lock()
	if id, ok := w.identity[bus]; ok {
		w.mu.Unlock()
		return id
	}
	w.mu.Unlock()

	obj := w.conn.Object(bus, ObjectPath)
	id := ShortName(bus)
	var v dbus.Variant
	if err := obj.CallWithContext(ctx, PropsIface+".Get", 0, RootIface, "Identity").Store(&v); err == nil {
		if s := variantString(v); s != "" {
			id = s
		}
	}
	w.mu.Lock()
	w.identity[bus] = id
	w.mu.Unlock()
	return id
}

func (w *Watcher) readState(ctx context.Context, bus string) (media.State, error) {
	obj := w.conn.Object(bus, ObjectPath)

	var props map[string]dbus.Variant
	err := obj.CallWithContext(ctx, PropsIface+".GetAll", 0, PlayerIface).Store(&props)
	if err != nil {
		return media.State{}, err
	}

	st := media.State{
		ID:       bus,
		Name:     ShortName(bus),
		Identity: w.identityOf(ctx, bus),
		At:       w.opts.Now(),
	}
	if v, ok := props["PlaybackStatus"]; ok {
		st.Status = media.Status(variantString(v))
	}
	if v, ok := props["Metadata"]; ok {
		if md, ok := v.Value().(map[string]dbus.Variant); ok {
			st.Track = parseMetadata(md)
		}
	}
	if v, ok := props["Position"]; ok {
		if us, ok := variantInt(v); ok && us >= 0 {
			st.Position = time.Duration(us) * time.Microsecond
		}
	} else {
		var v dbus.Variant
		if err := obj.CallWithContext(ctx, PropsIface+".Get", 0, PlayerIface, "Position").Store(&v); err == nil {
			if us, ok := variantInt(v); ok && us >= 0 {
				st.Position = time.Duration(us) * time.Microsecond
			}
		}
	}
	return st, nil
}

func (w *Watcher) emitState(ctx context.Context, bus string) {
	st, err := w.readState(ctx, bus)
	if err != nil {
		w.opts.Log.Debug("не удалось прочитать состояние плеера", "player", ShortName(bus), "err", err)
		return
	}
	w.send(ctx, st)
}

func (w *Watcher) send(ctx context.Context, st media.State) {
	select {
	case w.events <- st:
	case <-ctx.Done():
	}
}

func ListPlayers(ctx context.Context) ([]media.State, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("подключение к сессионной шине D-Bus: %w", err)
	}
	defer conn.Close()

	w := &Watcher{
		opts:     Options{Poll: time.Second, Log: slog.Default(), Now: time.Now},
		conn:     conn,
		owners:   make(map[string]string),
		buses:    make(map[string]string),
		identity: make(map[string]string),
	}

	var names []string
	if err := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		return nil, fmt.Errorf("получение списка имён D-Bus: %w", err)
	}

	var out []media.State
	for _, name := range names {
		if !strings.HasPrefix(name, BusPrefix) {
			continue
		}
		st, err := w.readState(ctx, name)
		if err != nil {
			continue
		}
		out = append(out, st)
	}
	return out, nil
}
