package lastfm

import "time"

type Track struct {
	Artist      string        `json:"artist"`
	Title       string        `json:"track"`
	Album       string        `json:"album,omitempty"`
	AlbumArtist string        `json:"album_artist,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	TrackNumber int           `json:"track_number,omitempty"`
	MBID        string        `json:"mbid,omitempty"`
}

func (t Track) Valid() bool {
	return t.Artist != "" && t.Title != ""
}

func (t Track) String() string {
	if t.Artist == "" {
		return t.Title
	}
	return t.Artist + " — " + t.Title
}

type Scrobble struct {
	Track     `json:"track"`
	Timestamp time.Time `json:"timestamp"`
	Radio     bool      `json:"radio,omitempty"`
}
