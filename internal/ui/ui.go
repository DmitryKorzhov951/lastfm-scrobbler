package ui

import (
	"context"
	"net/http"
	"time"
)

type Backend interface {
	State() State
	Queue() []QueueItem
	DeleteQueued(ids []int64) error
	ClearQueue() error
	FlushQueue()
	SetPaused(paused bool)

	Settings() Settings
	SaveSettings(ctx context.Context, s Settings) error

	SaveKeys(apiKey, secret string) error
	StartLogin(ctx context.Context) (authURL string, err error)
	CheckLogin(ctx context.Context) (done bool, err error)
	Logout() error

	Open(what string) error

	Stations() []Station
	AddStation(name, url string) (Station, error)
	EditStation(id, name, url string) error
	DeleteStation(id string) error
	PlayRadio(id string) error
	StopRadio()
	SetRadioVolume(volume int) error
	RadioStream(w http.ResponseWriter, r *http.Request, id string)
}

type State struct {
	HasKeys  bool        `json:"hasKeys"`
	LoggedIn bool        `json:"loggedIn"`
	User     string      `json:"user"`
	Since    time.Time   `json:"since"`
	Login    *LoginState `json:"login,omitempty"`
	Paused   bool        `json:"paused"`
	Now      *NowPlaying `json:"now,omitempty"`
	Link     Link        `json:"link"`
	QueueLen int         `json:"queueLen"`
	Today    Totals      `json:"today"`
	All      Totals      `json:"all"`
	Recent   []Scrobble  `json:"recent"`
	Radio    RadioState  `json:"radio"`
}

type RadioState struct {
	Playing   bool   `json:"playing"`
	Station   string `json:"station"`
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Song      string `json:"song"`
	Format    string `json:"format"`
	Error     string `json:"error,omitempty"`
	Volume    int    `json:"volume"`
	Last      string `json:"last"`
	Count     int    `json:"count"`
}

type Station struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type LoginState struct {
	URL   string `json:"url"`
	Error string `json:"error,omitempty"`
}

type NowPlaying struct {
	Player      string  `json:"player"`
	Status      string  `json:"status"`
	Artist      string  `json:"artist"`
	Title       string  `json:"title"`
	Album       string  `json:"album"`
	Length      float64 `json:"length"`
	Played      float64 `json:"played"`
	Target      float64 `json:"target"`
	Scrobbled   bool    `json:"scrobbled"`
	Scrobblable bool    `json:"scrobblable"`
	Live        bool    `json:"live"`
	FromStart   bool    `json:"fromStart"`
}

type Link struct {
	State   string    `json:"state"`
	Error   string    `json:"error,omitempty"`
	RetryAt time.Time `json:"retryAt,omitempty"`
}

type Totals struct {
	Count   int     `json:"count"`
	Seconds float64 `json:"seconds"`
}

type Scrobble struct {
	ID      int64     `json:"id"`
	Started time.Time `json:"started"`
	Artist  string    `json:"artist"`
	Title   string    `json:"title"`
	Status  string    `json:"status"`
	Reason  string    `json:"reason,omitempty"`
}

type QueueItem struct {
	ID       int64     `json:"id"`
	Started  time.Time `json:"started"`
	Artist   string    `json:"artist"`
	Title    string    `json:"title"`
	Attempts int       `json:"attempts"`
}

type Settings struct {
	Percent      float64         `json:"percent"`
	MaxPlay      float64         `json:"maxPlay"`
	MinLength    float64         `json:"minLength"`
	Radio        bool            `json:"radio"`
	Players      []PlayerSetting `json:"players"`
	Autostart    bool            `json:"autostart"`
	CanAutostart bool            `json:"canAutostart"`
	LogDir       string          `json:"logDir"`
}

type PlayerSetting struct {
	Name     string `json:"name"`
	Identity string `json:"identity"`
	Enabled  bool   `json:"enabled"`
	Playing  bool   `json:"playing"`
}
