//go:build !windows

package mpris

import (
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

const (
	BusPrefix   = "org.mpris.MediaPlayer2."
	ObjectPath  = dbus.ObjectPath("/org/mpris/MediaPlayer2")
	RootIface   = "org.mpris.MediaPlayer2"
	PlayerIface = "org.mpris.MediaPlayer2.Player"
	PropsIface  = "org.freedesktop.DBus.Properties"
)

func ShortName(bus string) string {
	return strings.TrimPrefix(bus, BusPrefix)
}

func BaseName(bus string) string {
	short := ShortName(bus)
	if i := strings.IndexByte(short, '.'); i >= 0 {
		return short[:i]
	}
	return short
}

func parseMetadata(md map[string]dbus.Variant) media.Track {
	var t media.Track
	for key, v := range md {
		switch key {
		case "mpris:trackid":
			t.TrackID = variantString(v)
		case "xesam:title":
			t.Title = strings.TrimSpace(variantString(v))
		case "xesam:artist":
			t.Artists = variantStrings(v)
		case "xesam:albumArtist":
			t.AlbumArtists = variantStrings(v)
		case "xesam:album":
			t.Album = strings.TrimSpace(variantString(v))
		case "mpris:length":
			if us, ok := variantInt(v); ok && us > 0 {
				t.Length = time.Duration(us) * time.Microsecond
			}
		case "xesam:trackNumber":
			if n, ok := variantInt(v); ok && n > 0 {
				t.TrackNumber = int(n)
			}
		case "xesam:musicBrainzTrackID", "mb:trackid", "xesam:musicBrainzTrackId":
			if t.MBID == "" {
				t.MBID = variantString(v)
			}
		case "xesam:url":
			t.URL = variantString(v)
		case "mpris:artUrl":
			t.ArtURL = variantString(v)
		}
	}
	return t
}

func variantString(v dbus.Variant) string {
	switch val := v.Value().(type) {
	case string:
		return val
	case dbus.ObjectPath:
		return string(val)
	case []string:
		if len(val) > 0 {
			return val[0]
		}
	case dbus.Variant:
		return variantString(val)
	}
	return ""
}

func variantStrings(v dbus.Variant) []string {
	switch val := v.Value().(type) {
	case []string:
		out := make([]string, 0, len(val))
		for _, s := range val {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if s := strings.TrimSpace(val); s != "" {
			return []string{s}
		}
	case dbus.Variant:
		return variantStrings(val)
	}
	return nil
}

func variantInt(v dbus.Variant) (int64, bool) {
	switch val := v.Value().(type) {
	case int64:
		return val, true
	case uint64:
		return int64(val), true
	case int32:
		return int64(val), true
	case uint32:
		return int64(val), true
	case int16:
		return int64(val), true
	case uint16:
		return int64(val), true
	case uint8:
		return int64(val), true
	case float64:
		return int64(val), true
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		return n, err == nil
	case dbus.Variant:
		return variantInt(val)
	}
	return 0, false
}
