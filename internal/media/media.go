package media

import (
	"strings"
	"time"
)

type Status string

const (
	Playing Status = "Playing"
	Paused  Status = "Paused"
	Stopped Status = "Stopped"
	Unknown Status = ""
)

type Track struct {
	Title        string
	Artists      []string
	AlbumArtists []string
	Album        string
	Length       time.Duration
	TrackNumber  int
	URL          string

	Live bool
}

func (t Track) Artist() string {
	return firstNonEmpty(t.Artists)
}

func (t Track) AlbumArtist() string {
	return firstNonEmpty(t.AlbumArtists)
}

func (t Track) Valid() bool {
	return t.Artist() != "" && strings.TrimSpace(t.Title) != ""
}

func (t Track) Key() string {
	return strings.Join([]string{t.Artist(), t.Title, t.Album}, "\x00")
}

func (t Track) String() string {
	if a := t.Artist(); a != "" {
		return a + " — " + t.Title
	}
	return t.Title
}

type State struct {
	ID       string
	Name     string
	Identity string
	Status   Status
	Track    Track
	Position time.Duration
	At       time.Time
	Gone     bool

	PositionFresh bool

	PositionKnown    bool
	PositionReported time.Time

	FromStart bool

	Art []byte
}

func firstNonEmpty(list []string) string {
	for _, s := range list {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}
