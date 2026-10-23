package aimp

import (
	"encoding/binary"
	"errors"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

const (
	RemoteName = "AIMP2_RemoteInfo"
	MapSize    = 2048

	WMCommand    = 0x0400 + 0x75
	StatusGet    = 1
	StatusPlayer = 4
	StatusPos    = 31
)

const (
	stateStopped = 0
	statePlaying = 1
	statePaused  = 2
)

const (
	offSize      = 0
	offActive    = 4
	offDuration  = 16
	offTrackNum  = 36
	offLengths   = 40
	headerSize32 = 88
	headerSize64 = 112
)

type Info struct {
	Active      bool
	Duration    time.Duration
	TrackNumber int
	Album       string
	Artist      string
	Date        string
	FileName    string
	Genre       string
	Title       string
}

func Parse(buf []byte) (Info, error) {
	if len(buf) < offLengths+6*4 {
		return Info{}, errors.New("AIMP: слишком короткие данные")
	}
	u32 := func(off int) uint32 { return binary.LittleEndian.Uint32(buf[off:]) }

	var lens [6]int
	total := 0
	for i := range lens {
		n := u32(offLengths + 4*i)
		if n > MapSize {
			return Info{}, errors.New("AIMP: неверная длина строки")
		}
		lens[i] = int(n)
		total += int(n)
	}

	for _, off := range candidates(int(u32(offSize))) {
		if off+2*total > len(buf) {
			continue
		}
		strs, ok := decode(buf[off:], lens)
		if !ok {
			continue
		}
		return Info{
			Active:      u32(offActive) != 0,
			Duration:    time.Duration(u32(offDuration)) * time.Millisecond,
			TrackNumber: int(u32(offTrackNum)),
			Album:       strs[0],
			Artist:      strs[1],
			Date:        strs[2],
			FileName:    strs[3],
			Genre:       strs[4],
			Title:       strs[5],
		}, nil
	}
	return Info{}, errors.New("AIMP: не удалось прочитать строки")
}

func candidates(declared int) []int {
	out := []int{headerSize32}
	if declared >= offLengths+6*4 && declared <= 256 && declared != headerSize32 {
		out = append(out, declared)
	}
	if declared != headerSize64 {
		out = append(out, headerSize64)
	}
	return out
}

func decode(b []byte, lens [6]int) ([6]string, bool) {
	var out [6]string
	pos := 0
	for i, n := range lens {
		u := make([]uint16, n)
		for j := range u {
			u[j] = binary.LittleEndian.Uint16(b[pos+2*j:])
			if u[j] == 0 {
				return out, false
			}
		}
		out[i] = string(utf16.Decode(u))
		pos += 2 * n
	}
	return out, true
}

func ToState(info Info, state int, pos time.Duration, at time.Time) media.State {
	t := media.Track{
		Title:       strings.TrimSpace(info.Title),
		Album:       strings.TrimSpace(info.Album),
		Length:      info.Duration,
		TrackNumber: info.TrackNumber,
		URL:         info.FileName,
	}
	artist := strings.TrimSpace(info.Artist)
	if artist == "" {
		if a, title, ok := strings.Cut(t.Title, " - "); ok && strings.TrimSpace(a) != "" {
			artist, t.Title = strings.TrimSpace(a), strings.TrimSpace(title)
		}
	}
	if artist != "" {
		t.Artists = []string{artist}
	}

	st := media.State{
		ID:            "aimp",
		Name:          "aimp",
		Identity:      "AIMP",
		Track:         t,
		Position:      pos,
		At:            at,
		PositionFresh: true,
	}
	switch state {
	case statePlaying:
		st.Status = media.Playing
	case statePaused:
		st.Status = media.Paused
	case stateStopped:
		st.Status = media.Stopped
	default:
		st.Status = media.Unknown
	}
	if st.Status != media.Playing {
		st.PositionFresh = false
	}
	if t.Length > 0 && st.Position > t.Length {
		st.Position = t.Length
	}
	return st
}
