package ui

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/library"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lyrics"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/moments"
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

	History(limit int) []Scrobble
	Phone() PhoneInfo
	SetPhone(enabled bool) (PhoneInfo, error)
	ResetPhone() (PhoneInfo, error)

	NowCover() string
	Cover(key string) (data []byte, contentType string, ok bool)

	Top(kind, period string, offset int) (library.Top, error)
	Card(kind, artist, title, album string) (library.Card, error)
	ReloadLibrary() error
	SyncLibrary()

	Lyrics() Lyrics
	SetClosed(panels []string) error
	SetMoment(r MomentRequest) error

	Recs() Recs
	OpenLastfm(artist, track string) error
}

const (
	RecsNone    = "none"
	RecsLoading = "loading"
	RecsFound   = "found"
	RecsNoKeys  = "nokeys"
	RecsError   = "error"
)

type Recs struct {
	State   string      `json:"state"`
	Artist  string      `json:"artist,omitempty"`
	Title   string      `json:"title,omitempty"`
	Artists []RecArtist `json:"artists"`
	Tracks  []RecTrack  `json:"tracks"`
}

type RecArtist struct {
	Name  string  `json:"name"`
	Match float64 `json:"match"`
	Plays int     `json:"plays"`
}

type RecTrack struct {
	Title  string  `json:"title"`
	Artist string  `json:"artist"`
	Match  float64 `json:"match"`
	Plays  int     `json:"plays"`
}

type Lyrics struct {
	Artist string `json:"artist"`
	Title  string `json:"title"`
	Album  string `json:"album,omitempty"`
	State  string `json:"state"`
	lyrics.Lyrics
}

const NoTrack = "none"

var Panels = []string{"now", "recs", "lyrics", "recent"}

func CleanPanels(list []string) ([]string, error) {
	want := map[string]bool{}
	for _, p := range list {
		if !slices.Contains(Panels, p) {
			return nil, errors.New("неизвестное окно: " + p)
		}
		want[p] = true
	}
	out := []string{}
	for _, p := range Panels {
		if want[p] {
			out = append(out, p)
		}
	}
	return out, nil
}

type State struct {
	HasKeys  bool           `json:"hasKeys"`
	LoggedIn bool           `json:"loggedIn"`
	User     string         `json:"user"`
	Since    time.Time      `json:"since"`
	Login    *LoginState    `json:"login,omitempty"`
	Paused   bool           `json:"paused"`
	Now      *NowPlaying    `json:"now,omitempty"`
	Link     Link           `json:"link"`
	QueueLen int            `json:"queueLen"`
	Today    Totals         `json:"today"`
	All      Totals         `json:"all"`
	Recent   []Scrobble     `json:"recent"`
	Radio    RadioState     `json:"radio"`
	Library  library.Status `json:"library"`
	Theme    string         `json:"theme"`
	Bursts   bool           `json:"bursts"`
	Closed   []string       `json:"closed"`
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
	Player      string          `json:"player"`
	Status      string          `json:"status"`
	Artist      string          `json:"artist"`
	Title       string          `json:"title"`
	Album       string          `json:"album"`
	Length      float64         `json:"length"`
	Played      float64         `json:"played"`
	Position    float64         `json:"position"`
	Synced      bool            `json:"synced"`
	Target      float64         `json:"target"`
	Left        float64         `json:"left"`
	Scrobbled   bool            `json:"scrobbled"`
	Scrobblable bool            `json:"scrobblable"`
	Live        bool            `json:"live"`
	FromStart   bool            `json:"fromStart"`
	Cover       string          `json:"cover,omitempty"`
	Charts      []library.Rank  `json:"charts,omitempty"`
	Loved       bool            `json:"loved,omitempty"`
	Moment      *moments.Moment `json:"moment,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
}

type MomentRequest struct {
	Artist string  `json:"artist"`
	Title  string  `json:"title"`
	Length float64 `json:"length"`
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Clear  bool    `json:"clear"`
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
	Album   string    `json:"album,omitempty"`
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
	Theme        string          `json:"theme"`
	Bursts       *bool           `json:"bursts,omitempty"`
	LyricsDir    string          `json:"lyricsDir"`
	LyricsAuto   string          `json:"lyricsAuto,omitempty"`
}

type PlayerSetting struct {
	Name     string `json:"name"`
	Identity string `json:"identity"`
	Enabled  bool   `json:"enabled"`
	Playing  bool   `json:"playing"`
}
