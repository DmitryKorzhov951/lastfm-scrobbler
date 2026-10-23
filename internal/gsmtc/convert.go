package gsmtc

import (
	"strings"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

const (
	statusClosed   int32 = 0
	statusOpened   int32 = 1
	statusChanging int32 = 2
	statusStopped  int32 = 3
	statusPlaying  int32 = 4
	statusPaused   int32 = 5
)

type Raw struct {
	AUMID       string
	Status      int32
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	TrackNumber int32
	Start       int64
	End         int64
	Position    int64
	Updated     int64
}

const freshWindow = 6 * time.Second

const windowsToUnix = 116444736000000000

func FromDateTime(v int64) time.Time {
	if v <= windowsToUnix {
		return time.Time{}
	}
	t := v - windowsToUnix
	return time.Unix(t/10_000_000, t%10_000_000*100).UTC()
}

func MapStatus(code int32) media.Status {
	switch code {
	case statusPlaying:
		return media.Playing
	case statusPaused, statusChanging:
		return media.Paused
	case statusStopped, statusOpened, statusClosed:
		return media.Stopped
	default:
		return media.Unknown
	}
}

func FromTimeSpan(ticks int64) time.Duration {
	return time.Duration(ticks) * 100
}

var aliases = map[string]string{
	"308046b0af4a39cb": "firefox",
	"spotifymusic":     "spotify",
}

var identities = map[string]string{
	"spotify":    "Spotify",
	"chrome":     "Google Chrome",
	"msedge":     "Microsoft Edge",
	"firefox":    "Firefox",
	"zunemusic":  "Медиаплеер Windows",
	"foobar2000": "foobar2000",
	"aimp":       "AIMP",
	"musicbee":   "MusicBee",
	"vlc":        "VLC",
}

func AppName(aumid string) string {
	s := strings.TrimSpace(aumid)
	if s == "" {
		return "unknown"
	}
	if alias, ok := aliases[strings.ToLower(s)]; ok {
		return alias
	}

	if i := strings.IndexByte(s, '!'); i >= 0 {
		family := s[:i]
		if j := strings.IndexByte(family, '_'); j >= 0 {
			family = family[:j]
		}
		if j := strings.LastIndexByte(family, '.'); j >= 0 {
			family = family[j+1:]
		}
		s = family
	} else {
		if j := strings.LastIndexAny(s, `\/`); j >= 0 {
			s = s[j+1:]
		}
		if strings.HasSuffix(strings.ToLower(s), ".exe") {
			s = s[:len(s)-4]
		}
	}

	name := strings.ToLower(s)
	if alias, ok := aliases[name]; ok {
		return alias
	}
	return name
}

func Identity(name string) string {
	if id, ok := identities[name]; ok {
		return id
	}
	return name
}

func Allowed(name string, allow, deny []string) bool {
	for _, d := range deny {
		if strings.EqualFold(strings.TrimSpace(d), name) {
			return false
		}
	}
	if len(allow) == 0 {
		return true
	}
	for _, a := range allow {
		if strings.EqualFold(strings.TrimSpace(a), name) {
			return true
		}
	}
	return false
}

func ToState(r Raw, at time.Time) media.State {
	name := AppName(r.AUMID)
	id := strings.TrimSpace(r.AUMID)
	if id == "" {
		id = name
	}

	t := media.Track{
		Title: strings.TrimSpace(r.Title),
		Album: strings.TrimSpace(r.Album),
	}
	if a := strings.TrimSpace(r.Artist); a != "" {
		t.Artists = []string{a}
	}
	if a := strings.TrimSpace(r.AlbumArtist); a != "" {
		t.AlbumArtists = []string{a}
	}
	if r.TrackNumber > 0 {
		t.TrackNumber = int(r.TrackNumber)
	}
	if r.End > r.Start {
		t.Length = FromTimeSpan(r.End - r.Start)
	}

	status := MapStatus(r.Status)
	pos := FromTimeSpan(r.Position - r.Start)
	if pos < 0 {
		pos = 0
	}

	fresh := false
	if updated := FromDateTime(r.Updated); !updated.IsZero() && status == media.Playing {
		age := at.Sub(updated)
		if age >= -time.Second && age <= freshWindow {
			fresh = true
			if age > 0 {
				pos += age
			}
		}
	}
	if t.Length > 0 && pos > t.Length {
		pos = t.Length
	}

	return media.State{
		ID:            id,
		Name:          name,
		Identity:      Identity(name),
		Status:        status,
		Track:         t,
		Position:      pos,
		At:            at,
		PositionFresh: fresh,
	}
}

func statusRank(code int32) int {
	switch code {
	case statusPlaying:
		return 3
	case statusPaused, statusChanging:
		return 2
	case statusStopped, statusOpened:
		return 1
	default:
		return 0
	}
}

func Pick(raws []Raw) []Raw {
	best := make(map[string]int, len(raws))
	out := make([]Raw, 0, len(raws))
	for _, r := range raws {
		key := strings.ToLower(strings.TrimSpace(r.AUMID))
		i, ok := best[key]
		if !ok {
			best[key] = len(out)
			out = append(out, r)
			continue
		}
		if statusRank(r.Status) > statusRank(out[i].Status) {
			out[i] = r
		}
	}
	return out
}
