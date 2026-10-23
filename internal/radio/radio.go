package radio

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/atomicfile"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

const (
	PlayerID       = "radio"
	PlayerName     = "radio"
	PlayerIdentity = "Радио"
)

const (
	maxStations   = 200
	maxNameLen    = 80
	maxURLLen     = 2048
	defaultVolume = 80

	listenTimeout = 30 * time.Second
)

type Station struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	AutoName bool   `json:"auto_name,omitempty"`
}

type Status struct {
	Playing   bool
	Station   Station
	Connected bool
	Title     string
	Artist    string
	Song      string
	Format    string
	Error     string
	Volume    int
	Last      string
}

type saved struct {
	Stations []Station `json:"stations"`
	Volume   int       `json:"volume"`
	Last     string    `json:"last,omitempty"`
}

type Radio struct {
	path      string
	log       *slog.Logger
	client    *http.Client
	userAgent string
	events    chan media.State
	now       func() time.Time

	mu        sync.Mutex
	data      saved
	playing   bool
	station   Station
	conn      *listener
	idleSince time.Time
	title     string
	artist    string
	song      string
	format    string
	errText   string
}

type listener struct {
	cancel    context.CancelFunc
	connected bool
	heard     bool
}

func Open(path string, log *slog.Logger, userAgent string) (*Radio, error) {
	if log == nil {
		log = slog.Default()
	}
	r := &Radio{
		path:      path,
		log:       log,
		client:    NewClient(),
		userAgent: userAgent,
		events:    make(chan media.State, 64),
		now:       time.Now,
		data:      saved{Volume: defaultVolume},
	}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return r, nil
	case err != nil:
		return r, err
	}
	if err := json.Unmarshal(data, &r.data); err != nil {
		_ = os.Rename(path, path+".broken")
		r.data = saved{Volume: defaultVolume}
		return r, fmt.Errorf("список станций испорчен и отложен в %s.broken: %w", path, err)
	}
	if r.data.Volume < 0 || r.data.Volume > 100 {
		r.data.Volume = defaultVolume
	}
	return r, nil
}

func (r *Radio) Events() <-chan media.State { return r.events }

func (r *Radio) Stations() []Station {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Station(nil), r.data.Stations...)
}

func (r *Radio) Add(name, rawURL string) (Station, error) {
	st, err := validate(name, rawURL)
	if err != nil {
		return st, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.data.Stations) >= maxStations {
		return st, fmt.Errorf("станций уже %d — удалите ненужные", maxStations)
	}
	st.ID = newID()
	r.data.Stations = append(r.data.Stations, st)
	return st, r.saveLocked()
}

func (r *Radio) Edit(id, name, rawURL string) error {
	st, err := validate(name, rawURL)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	i := r.indexLocked(id)
	if i < 0 {
		return errors.New("станция не найдена")
	}
	st.ID = id
	old := r.data.Stations[i]
	r.data.Stations[i] = st
	if r.playing && r.station.ID == id {
		r.station = st
		if old.URL != st.URL && r.conn != nil {
			r.conn.cancel()
		}
	}
	return r.saveLocked()
}

func (r *Radio) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	i := r.indexLocked(id)
	if i < 0 {
		return errors.New("станция не найдена")
	}
	r.data.Stations = append(r.data.Stations[:i], r.data.Stations[i+1:]...)
	if r.data.Last == id {
		r.data.Last = ""
	}
	if r.playing && r.station.ID == id {
		r.stopLocked("")
	}
	return r.saveLocked()
}

func (r *Radio) Play(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	i := r.indexLocked(id)
	if i < 0 {
		return errors.New("станция не найдена")
	}
	if r.playing && r.station.ID == id {
		return nil
	}
	r.stopLocked("")
	r.playing = true
	r.station = r.data.Stations[i]
	r.idleSince = r.now()
	r.data.Last = id
	r.log.Info("радио включено", "station", r.station.Name)
	return r.saveLocked()
}

func (r *Radio) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.playing {
		r.log.Info("радио выключено", "station", r.station.Name)
	}
	r.stopLocked("")
}

func (r *Radio) SetVolume(v int) error {
	v = max(0, min(100, v))
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.data.Volume == v {
		return nil
	}
	r.data.Volume = v
	return r.saveLocked()
}

func (r *Radio) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Status{
		Playing:   r.playing,
		Station:   r.station,
		Connected: r.conn != nil && r.conn.connected,
		Title:     r.title,
		Artist:    r.artist,
		Song:      r.song,
		Format:    r.format,
		Error:     r.errText,
		Volume:    r.data.Volume,
		Last:      r.data.Last,
	}
}

func (r *Radio) LastStation() (Station, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i := r.indexLocked(r.data.Last); i >= 0 {
		return r.data.Stations[i], true
	}
	if len(r.data.Stations) > 0 {
		return r.data.Stations[0], true
	}
	return Station{}, false
}

func (r *Radio) Run(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			r.Stop()
			return
		case <-t.C:
			r.mu.Lock()
			if r.playing && r.conn == nil && r.now().Sub(r.idleSince) > listenTimeout {
				r.log.Info("радио выключено: окно его не слушает")
				r.stopLocked("радио выключено: окно программы закрыли")
			}
			r.mu.Unlock()
		}
	}
}

func (r *Radio) ServeStream(w http.ResponseWriter, req *http.Request, id string) {
	r.mu.Lock()
	if !r.playing || r.station.ID != id {
		r.mu.Unlock()
		http.Error(w, "радио выключено", http.StatusConflict)
		return
	}
	station := r.station
	if r.conn != nil {
		r.conn.cancel()
	}
	ctx, cancel := context.WithCancel(req.Context())
	l := &listener{cancel: cancel}
	r.conn = l
	r.errText = ""
	r.mu.Unlock()

	defer func() {
		cancel()
		r.mu.Lock()
		if r.conn == l {
			r.conn = nil
			r.idleSince = r.now()
			if r.playing {
				r.emitLocked(media.Stopped, false)
			}
		}
		r.mu.Unlock()
	}()

	s, err := Connect(ctx, r.client, station.URL, r.userAgent, func(t string) { r.onTitle(l, station, t) })
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		r.log.Warn("радио: не удалось подключиться", "station", station.Name, "err", err)
		r.mu.Lock()
		if r.conn == l {
			r.stopLocked(err.Error())
		}
		r.mu.Unlock()
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer s.Close()

	r.mu.Lock()
	if r.conn != l {
		r.mu.Unlock()
		return
	}
	l.connected = true
	r.format = s.Format()
	if station.AutoName && s.Name != "" {
		r.renameLocked(station.ID, s.Name)
		station = r.station
	}
	if r.title == "" {
		r.song = station.Name
	}
	r.emitLocked(media.Playing, false)
	r.mu.Unlock()
	r.log.Info("радио: звук идёт в окно", "station", station.Name, "format", s.Format())

	h := w.Header()
	h.Set("Content-Type", s.ContentType)
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 16<<10)
	for {
		n, err := s.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			if ctx.Err() == nil {
				r.log.Warn("радио: станция прервала поток", "station", station.Name, "err", err)
				r.mu.Lock()
				if r.conn == l {
					r.errText = "станция прервала вещание — переподключаюсь"
				}
				r.mu.Unlock()
			}
			return
		}
	}
}

func (r *Radio) onTitle(l *listener, station Station, raw string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn != l {
		return
	}
	fromStart := l.heard
	l.heard = true
	r.title = raw
	artist, song, ok := SplitTitle(raw)
	if !ok || strings.EqualFold(artist, station.Name) || strings.EqualFold(song, station.Name) {
		artist, song = "", raw
	}
	if song == "" {
		song = station.Name
	}
	r.artist, r.song = artist, song
	r.log.Debug("радио: в эфире", "station", station.Name, "title", raw)
	r.emitLocked(media.Playing, fromStart)
}

func (r *Radio) stopLocked(reason string) {
	wasOn := r.playing
	r.playing = false
	if r.conn != nil {
		r.conn.cancel()
		r.conn = nil
	}
	r.title, r.artist, r.song, r.format = "", "", "", ""
	r.errText = reason
	if wasOn {
		r.send(media.State{ID: PlayerID, Name: PlayerName, Identity: PlayerIdentity, Gone: true, At: r.now()})
	}
}

func (r *Radio) emitLocked(status media.Status, fromStart bool) {
	tr := media.Track{Title: r.song, Live: true}
	if r.artist != "" {
		tr.Artists = []string{r.artist}
	}
	r.send(media.State{
		ID:            PlayerID,
		Name:          PlayerName,
		Identity:      PlayerIdentity,
		Status:        status,
		Track:         tr,
		At:            r.now(),
		PositionFresh: true,
		FromStart:     fromStart,
	})
}

func (r *Radio) send(st media.State) {
	select {
	case r.events <- st:
	default:
		r.log.Warn("радио: очередь событий переполнена")
	}
}

func (r *Radio) renameLocked(id, name string) {
	name = strings.Join(strings.Fields(name), " ")
	if utf8.RuneCountInString(name) > maxNameLen {
		name = string([]rune(name)[:maxNameLen])
	}
	i := r.indexLocked(id)
	if i < 0 || name == "" {
		return
	}
	r.data.Stations[i].Name = name
	r.data.Stations[i].AutoName = false
	if r.station.ID == id {
		r.station = r.data.Stations[i]
	}
	if err := r.saveLocked(); err != nil {
		r.log.Warn("не удалось сохранить название станции", "err", err)
	}
}

func (r *Radio) indexLocked(id string) int {
	for i, s := range r.data.Stations {
		if s.ID == id {
			return i
		}
	}
	return -1
}

func (r *Radio) saveLocked() error {
	data, err := json.MarshalIndent(r.data, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(r.path, append(data, '\n'))
}

func validate(name, rawURL string) (Station, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return Station{}, errors.New("укажите адрес потока станции")
	}
	if len(rawURL) > maxURLLen {
		return Station{}, errors.New("слишком длинный адрес")
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return Station{}, errors.New("адрес должен начинаться с http:// или https://")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		return Station{}, errors.New("адрес должен начинаться с http:// или https://")
	}
	name = strings.Join(strings.Fields(name), " ")
	auto := name == ""
	if auto {
		name = u.Hostname()
	}
	if utf8.RuneCountInString(name) > maxNameLen {
		name = string([]rune(name)[:maxNameLen])
	}
	return Station{Name: name, URL: u.String(), AutoName: auto}, nil
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
