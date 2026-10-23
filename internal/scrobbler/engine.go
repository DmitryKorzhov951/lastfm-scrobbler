package scrobbler

import (
	"log/slog"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lastfm"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

type Options struct {
	MinTrackLength    time.Duration
	MaxPlayTime       time.Duration
	Percent           float64
	NowPlayingRefresh time.Duration
	MaxGap            time.Duration
	SkipUnknownLength bool

	IgnoreInitialPosition bool
}

func DefaultOptions() Options {
	return Options{
		MinTrackLength:    30 * time.Second,
		MaxPlayTime:       4 * time.Minute,
		Percent:           0.5,
		NowPlayingRefresh: 4 * time.Minute,
		MaxGap:            15 * time.Second,
		SkipUnknownLength: false,
	}
}

func (o *Options) normalize() {
	d := DefaultOptions()
	if o.MinTrackLength <= 0 {
		o.MinTrackLength = d.MinTrackLength
	}
	if o.MaxPlayTime <= 0 {
		o.MaxPlayTime = d.MaxPlayTime
	}
	if o.Percent <= 0 || o.Percent > 1 {
		o.Percent = d.Percent
	}
	if o.NowPlayingRefresh <= 0 {
		o.NowPlayingRefresh = d.NowPlayingRefresh
	}
	if o.MaxGap <= 0 {
		o.MaxGap = d.MaxGap
	}
}

type Sink interface {
	NowPlaying(t lastfm.Track)
	Scrobble(s lastfm.Scrobble)
}

type playerState struct {
	id       string
	name     string
	identity string

	status    media.Status
	track     media.Track
	trackKey  string
	startedAt time.Time
	played    time.Duration
	lastTick  time.Time
	lastPos   time.Duration
	nowPlayAt time.Time
	scrobbled bool
	fromStart bool
}

type Engine struct {
	opts    Options
	sink    Sink
	log     *slog.Logger
	players map[string]*playerState
	active  string
}

func New(opts Options, sink Sink, log *slog.Logger) *Engine {
	opts.normalize()
	if log == nil {
		log = slog.Default()
	}
	return &Engine{
		opts:    opts,
		sink:    sink,
		log:     log,
		players: make(map[string]*playerState),
	}
}

func (e *Engine) SetOptions(opts Options) {
	opts.normalize()
	e.opts = opts
}

func (e *Engine) Update(st media.State) {
	if st.Gone {
		e.remove(st.ID)
		return
	}

	ps, ok := e.players[st.ID]
	if !ok {
		ps = &playerState{id: st.ID, name: st.Name, identity: st.Identity, lastTick: st.At}
		e.players[st.ID] = ps
	}
	ps.name, ps.identity = st.Name, st.Identity

	e.advance(ps, st.At)

	key := st.Track.Key()
	switch {
	case key != ps.trackKey:
		e.finishLive(ps, st.At)
		e.startTrack(ps, st)
	case e.restarted(ps, st):
		e.log.Debug("трек начат заново", "player", ps.name, "track", st.Track.String())
		e.startTrack(ps, st)
	default:
		ps.track = st.Track
		e.catchUp(ps, st)
	}
	ps.lastPos = st.Position

	prev := ps.status
	ps.status = st.Status

	if st.Status == media.Playing {
		if prev != media.Playing || e.active == "" || !e.isPlaying(e.active) {
			if e.active != st.ID {
				e.log.Debug("активный плеер изменился", "player", ps.name)
			}
			e.active = st.ID
		}
	} else if e.active == st.ID && st.Status == media.Stopped {
		e.active = ""
	}

	e.check(ps, st.At)
}

func (e *Engine) Tick(now time.Time) {
	for _, ps := range e.players {
		e.advance(ps, now)
		e.check(ps, now)
	}
}

type Snapshot struct {
	Player    string
	Identity  string
	Active    bool
	Status    media.Status
	Track     media.Track
	Played    time.Duration
	Target    time.Duration
	Scrobbld  bool
	FromStart bool
}

func (e *Engine) Snapshots() []Snapshot {
	out := make([]Snapshot, 0, len(e.players))
	for id, ps := range e.players {
		target, _ := e.threshold(ps.track)
		out = append(out, Snapshot{
			Player:    ps.name,
			Identity:  ps.identity,
			Active:    id == e.active,
			Status:    ps.status,
			Track:     ps.track,
			Played:    ps.played,
			Target:    target,
			Scrobbld:  ps.scrobbled,
			FromStart: ps.fromStart,
		})
	}
	return out
}

func (e *Engine) remove(id string) {
	delete(e.players, id)
	if e.active == id {
		e.active = ""
	}
}

func (e *Engine) isPlaying(id string) bool {
	ps, ok := e.players[id]
	return ok && ps.status == media.Playing
}

func (e *Engine) startTrack(ps *playerState, st media.State) {
	ps.track = st.Track
	ps.trackKey = st.Track.Key()
	ps.played = 0
	ps.scrobbled = false
	ps.nowPlayAt = time.Time{}
	ps.lastTick = st.At
	ps.fromStart = st.Track.Live && st.FromStart

	if e.opts.IgnoreInitialPosition && !(st.PositionFresh && st.Position <= maxCatchUp) {
		ps.startedAt = st.At
		return
	}

	started := st.At.Add(-st.Position)
	if st.Position < 0 || st.Position > 12*time.Hour {
		started = st.At
	}
	ps.startedAt = started
	if st.Position > 0 && st.Status == media.Playing {
		ps.played = st.Position
	}
}

func (e *Engine) finishLive(ps *playerState, now time.Time) {
	if !ps.track.Live || !ps.fromStart || ps.scrobbled || !ps.track.Valid() {
		return
	}
	if ps.status != media.Playing || e.active != ps.id || ps.played < e.opts.MinTrackLength {
		return
	}
	ps.scrobbled = true
	t := toLastfm(ps.track)
	t.Duration = ps.played.Round(time.Second)
	ts := ps.startedAt
	if ts.After(now) || ts.IsZero() {
		ts = now
	}
	e.log.Info("песня с радио засчитана",
		"player", ps.name,
		"track", ps.track.String(),
		"played", ps.played.Round(time.Second))
	e.sink.Scrobble(lastfm.Scrobble{Track: t, Timestamp: ts.UTC(), Radio: true})
}

const maxCatchUp = 10 * time.Second

func (e *Engine) catchUp(ps *playerState, st media.State) {
	if !e.opts.IgnoreInitialPosition || !st.PositionFresh || ps.scrobbled || st.Status != media.Playing {
		return
	}
	if lag := st.Position - ps.played; lag > 0 && lag <= maxCatchUp {
		ps.played = st.Position
		if started := st.At.Add(-st.Position); started.Before(ps.startedAt) {
			ps.startedAt = started
		}
	}
}

func (e *Engine) restarted(ps *playerState, st media.State) bool {
	if !ps.scrobbled {
		return false
	}
	return st.Position < 3*time.Second && ps.lastPos > st.Position+5*time.Second
}

func (e *Engine) advance(ps *playerState, now time.Time) {
	if ps.lastTick.IsZero() {
		ps.lastTick = now
		return
	}
	d := now.Sub(ps.lastTick)
	ps.lastTick = now
	if d <= 0 {
		return
	}
	if d > e.opts.MaxGap {
		d = e.opts.MaxGap
	}
	if ps.status == media.Playing && e.active == ps.id {
		ps.played += d
	}
}

func (e *Engine) threshold(t media.Track) (time.Duration, bool) {
	if t.Length <= 0 {
		if e.opts.SkipUnknownLength {
			return 0, false
		}
		return e.opts.MaxPlayTime, true
	}
	if t.Length < e.opts.MinTrackLength {
		return 0, false
	}
	target := time.Duration(float64(t.Length) * e.opts.Percent)
	if target > e.opts.MaxPlayTime {
		target = e.opts.MaxPlayTime
	}
	return target, true
}

func (e *Engine) check(ps *playerState, now time.Time) {
	if ps.status != media.Playing || e.active != ps.id || !ps.track.Valid() {
		return
	}

	if ps.nowPlayAt.IsZero() || now.Sub(ps.nowPlayAt) >= e.opts.NowPlayingRefresh {
		ps.nowPlayAt = now
		e.sink.NowPlaying(toLastfm(ps.track))
	}

	if ps.scrobbled {
		return
	}
	target, ok := e.threshold(ps.track)
	if !ok || ps.played < target {
		return
	}

	ps.scrobbled = true
	ts := ps.startedAt
	if ts.After(now) || ts.IsZero() {
		ts = now
	}
	e.log.Info("трек засчитан",
		"player", ps.name,
		"track", ps.track.String(),
		"played", ps.played.Round(time.Second),
		"target", target.Round(time.Second))
	e.sink.Scrobble(lastfm.Scrobble{Track: toLastfm(ps.track), Timestamp: ts.UTC(), Radio: ps.track.Live})
}

func toLastfm(t media.Track) lastfm.Track {
	return lastfm.Track{
		Artist:      t.Artist(),
		Title:       t.Title,
		Album:       t.Album,
		AlbumArtist: t.AlbumArtist(),
		Duration:    t.Length,
		TrackNumber: t.TrackNumber,
		MBID:        t.MBID,
	}
}
