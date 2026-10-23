package ui

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Demo struct {
	mu       sync.Mutex
	loggedIn bool
	offline  bool
	paused   bool
	start    time.Time
	queue    []QueueItem
	recent   []Scrobble
	settings Settings
	waiting  time.Time

	stations  []Station
	radioOn   bool
	radioID   string
	radioLast string
	volume    int
	listening int
	nextID    int
}

func NewDemo(loggedIn, offline bool) *Demo {
	now := time.Now()
	at := func(h, m int) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, time.Local)
	}
	d := &Demo{loggedIn: loggedIn, offline: offline, start: now}
	d.recent = []Scrobble{
		{ID: 1284, Started: at(18, 41), Artist: "Lana Del Rey", Title: "Summertime Sadness", Status: "sent"},
		{ID: 1283, Started: at(18, 37), Artist: "MGK", Title: "bloody valentine", Status: "sent"},
		{ID: 1282, Started: at(18, 34), Artist: "Ninajirachi", Title: "iPod Touch", Status: "sent"},
		{ID: 1281, Started: at(18, 32), Artist: "Charli XCX", Title: "360", Status: "sent"},
	}
	if offline {
		d.queue = []QueueItem{
			{ID: 1288, Started: at(19, 1), Artist: "Charli XCX", Title: "Von dutch", Attempts: 2},
			{ID: 1287, Started: at(18, 59), Artist: "MGK", Title: "my ex's best friend", Attempts: 2},
			{ID: 1286, Started: at(18, 55), Artist: "Lana Del Rey", Title: "Young and Beautiful", Attempts: 3},
			{ID: 1285, Started: at(18, 51), Artist: "Ninajirachi", Title: "All I Am", Attempts: 4},
		}
	}
	d.settings = Settings{
		Percent: 0.5, MaxPlay: 240, MinLength: 30, Radio: true,
		Players: []PlayerSetting{
			{Name: "spotify", Identity: "Spotify", Enabled: true, Playing: true},
			{Name: "aimp", Identity: "AIMP", Enabled: true},
			{Name: "firefox", Identity: "Firefox", Enabled: true},
		},
		Autostart: true, CanAutostart: true, LogDir: `%LOCALAPPDATA%\lastfm-scrobbler`,
	}
	d.stations = []Station{
		{ID: "1", Name: "Groove Salad (SomaFM)", URL: "https://ice2.somafm.com/groovesalad-128-mp3"},
		{ID: "2", Name: "Радио Рекорд", URL: "https://radiorecord.hostingradio.ru/rr_main96.aacp"},
		{ID: "3", Name: "Nightwave Plaza", URL: "https://radio.plaza.one/mp3"},
	}
	d.nextID = 4
	d.volume = 80
	d.radioLast = "1"
	return d
}

func (d *Demo) State() State {
	d.mu.Lock()
	defer d.mu.Unlock()
	played := 62 + time.Since(d.start).Seconds()
	if played > 215 {
		played = 215
	}
	link := Link{State: "ok"}
	if d.offline {
		link = Link{State: "offline", Error: "dial tcp: no route to host", RetryAt: time.Now().Add(2 * time.Minute)}
	}
	if !d.loggedIn {
		link = Link{State: "nologin"}
	}
	st := State{
		HasKeys:  d.loggedIn,
		LoggedIn: d.loggedIn,
		User:     "listener",
		Since:    time.Date(2026, 10, 12, 20, 0, 0, 0, time.Local),
		Paused:   d.paused,
		Now: &NowPlaying{
			Player: "Spotify", Status: "playing",
			Artist: "100 gecs", Title: "ringtone (remix) [feat. Charli XCX, Rico Nasty, Kero Kero Bonito]",
			Album: "1000 gecs and the Tree of Clues", Length: 215, Played: played, Target: 107.5,
			Scrobbled: played >= 107.5, Scrobblable: true,
		},
		Link:     link,
		QueueLen: len(d.queue),
		Today:    Totals{Count: 23, Seconds: 83 * 60},
		All:      Totals{Count: 1284, Seconds: 1284 * 200},
		Recent:   append([]Scrobble(nil), d.recent...),
	}
	if !d.waiting.IsZero() {
		st.Login = &LoginState{URL: "https://www.last.fm/api/auth/"}
	}
	st.Radio = RadioState{Volume: d.volume, Last: d.radioLast, Count: len(d.stations)}
	if d.radioOn {
		for _, s := range d.stations {
			if s.ID == d.radioID {
				st.Radio.Name = s.Name
			}
		}
		st.Radio.Playing, st.Radio.Station = true, d.radioID
		st.Radio.Connected = d.listening > 0
		st.Radio.Title, st.Radio.Artist, st.Radio.Song = "Ninajirachi - All I Am", "Ninajirachi", "All I Am"
		st.Radio.Format = "MP3 · 128 кбит/с"
	}
	return st
}

func (d *Demo) Queue() []QueueItem {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]QueueItem(nil), d.queue...)
}

func (d *Demo) DeleteQueued(ids []int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	drop := map[int64]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	kept := d.queue[:0:0]
	for _, q := range d.queue {
		if !drop[q.ID] {
			kept = append(kept, q)
		}
	}
	d.queue = kept
	return nil
}

func (d *Demo) ClearQueue() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.queue = nil
	return nil
}

func (d *Demo) FlushQueue() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.offline = false
	for _, q := range d.queue {
		d.recent = append([]Scrobble{{ID: q.ID, Started: q.Started, Artist: q.Artist, Title: q.Title, Status: "sent"}}, d.recent...)
	}
	d.queue = nil
}

func (d *Demo) SetPaused(p bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.paused = p
}

func (d *Demo) Settings() Settings {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.settings
	s.Players = append([]PlayerSetting(nil), d.settings.Players...)
	return s
}

func (d *Demo) SaveSettings(_ context.Context, s Settings) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.settings = s
	return nil
}

func (d *Demo) SaveKeys(key, secret string) error {
	if len(key) != 32 || len(secret) != 32 {
		return errors.New("ключ должен состоять из 32 символов 0–9 и a–f — скопируйте его со страницы Last.fm целиком")
	}
	return nil
}

func (d *Demo) StartLogin(context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.waiting = time.Now()
	return "https://www.last.fm/api/auth/", nil
}

func (d *Demo) CheckLogin(context.Context) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.waiting.IsZero() {
		return false, errors.New("вход не начат")
	}
	if time.Since(d.waiting) < 3*time.Second {
		return false, nil
	}
	d.waiting = time.Time{}
	d.loggedIn = true
	return true, nil
}

func (d *Demo) Logout() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.loggedIn = false
	return nil
}

func (d *Demo) Open(string) error { return nil }

func (d *Demo) Stations() []Station {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]Station(nil), d.stations...)
}

func demoStation(name, raw string) (Station, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Station{}, errors.New("адрес должен начинаться с http:// или https://")
	}
	if name = strings.TrimSpace(name); name == "" {
		name = u.Hostname()
	}
	return Station{Name: name, URL: u.String()}, nil
}

func (d *Demo) AddStation(name, raw string) (Station, error) {
	st, err := demoStation(name, raw)
	if err != nil {
		return st, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	st.ID = strconv.Itoa(d.nextID)
	d.nextID++
	d.stations = append(d.stations, st)
	return st, nil
}

func (d *Demo) EditStation(id, name, raw string) error {
	st, err := demoStation(name, raw)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.stations {
		if d.stations[i].ID == id {
			st.ID = id
			d.stations[i] = st
			return nil
		}
	}
	return errors.New("станция не найдена")
}

func (d *Demo) DeleteStation(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.stations {
		if d.stations[i].ID == id {
			d.stations = append(d.stations[:i], d.stations[i+1:]...)
			if d.radioID == id {
				d.radioOn = false
			}
			return nil
		}
	}
	return errors.New("станция не найдена")
}

func (d *Demo) PlayRadio(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range d.stations {
		if s.ID == id {
			d.radioOn, d.radioID, d.radioLast = true, id, id
			return nil
		}
	}
	return errors.New("станция не найдена")
}

func (d *Demo) StopRadio() {
	d.mu.Lock()
	d.radioOn = false
	d.mu.Unlock()
}

func (d *Demo) SetRadioVolume(v int) error {
	d.mu.Lock()
	d.volume = v
	d.mu.Unlock()
	return nil
}

func silentMP3() []byte {
	frame := make([]byte, 417)
	copy(frame, []byte{0xFF, 0xFB, 0x90, 0xC0})
	return bytes.Repeat(frame, 39)
}

func (d *Demo) RadioStream(w http.ResponseWriter, r *http.Request, id string) {
	d.mu.Lock()
	if !d.radioOn || d.radioID != id {
		d.mu.Unlock()
		http.Error(w, "радио выключено", http.StatusConflict)
		return
	}
	d.listening++
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		d.listening--
		d.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-store")
	flusher, _ := w.(http.Flusher)
	second := silentMP3()
	for i := 0; ; i++ {
		if _, err := w.Write(second); err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
		if i < 3 {
			continue
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(time.Second):
		}
	}
}
