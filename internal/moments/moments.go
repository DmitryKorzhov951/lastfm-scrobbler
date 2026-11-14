package moments

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/atomicfile"
)

const (
	StartZone = 12.0

	seekSlack  = 3.0
	jumpSpan   = 15
	jumpWeight = 0.5
	window     = 10
	minHeat    = 8.0
	minMoment  = 8
	maxMoment  = 40
	maxLength  = 2 * 3600.0
	maxTracks  = 1000
	fileVer    = 1
)

type Moment struct {
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Manual bool    `json:"manual,omitempty"`
}

type Sample struct {
	Player   string
	Artist   string
	Title    string
	Length   float64
	Position float64
	Playing  bool
}

type track struct {
	Artist string          `json:"artist"`
	Title  string          `json:"title"`
	Length float64         `json:"length"`
	Heat   map[int]float64 `json:"heat,omitempty"`
	Manual *Moment         `json:"manual,omitempty"`
	Seen   time.Time       `json:"seen"`
}

type stored struct {
	Version int               `json:"version"`
	Tracks  map[string]*track `json:"tracks"`
}

type session struct {
	key     string
	artist  string
	title   string
	length  float64
	pos     float64
	at      time.Time
	playing bool
	counts  []uint8
	jumps   []float64
}

type Store struct {
	Log *slog.Logger

	path string

	mu       sync.Mutex
	tracks   map[string]*track
	sessions map[string]*session
}

func Open(path string, log *slog.Logger) (*Store, error) {
	if log == nil {
		log = slog.Default()
	}
	s := &Store{Log: log, path: path, tracks: map[string]*track{}, sessions: map[string]*session{}}
	if path == "" {
		return s, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	var f stored
	if err := json.Unmarshal(data, &f); err != nil {
		return s, fmt.Errorf("moments.json: %w", err)
	}
	for k, t := range f.Tracks {
		if t != nil {
			s.tracks[k] = t
		}
	}
	return s, nil
}

func Key(artist, title string) string {
	return norm(artist) + "\x00" + norm(title)
}

func norm(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func (s *Store) Observe(samples []Sample, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]bool{}
	for _, sm := range samples {
		if sm.Player == "" || sm.Length <= 0 || sm.Length > maxLength || norm(sm.Artist) == "" || norm(sm.Title) == "" {
			continue
		}
		seen[sm.Player] = true
		s.observe(sm, now)
	}
	for id, ses := range s.sessions {
		if !seen[id] {
			s.finish(ses, now)
			delete(s.sessions, id)
		}
	}
}

func (s *Store) observe(sm Sample, now time.Time) {
	key := Key(sm.Artist, sm.Title)
	ses := s.sessions[sm.Player]
	if ses != nil && ses.key != key {
		s.finish(ses, now)
		ses = nil
	}
	if ses == nil {
		s.sessions[sm.Player] = newSession(key, sm, now)
		return
	}
	expected := ses.pos
	if ses.playing {
		expected += now.Sub(ses.at).Seconds()
	}
	diff := sm.Position - expected
	switch {
	case math.Abs(diff) <= seekSlack:
		if ses.playing && sm.Position > ses.pos {
			ses.mark(ses.pos, sm.Position)
		}
	case diff < 0 && sm.Position < StartZone:
		s.Log.Debug("трек начат заново — это не переслушивание", "track", ses.name())
		s.finish(ses, now)
		s.sessions[sm.Player] = newSession(key, sm, now)
		return
	case diff < 0:
		s.Log.Debug("перемотка назад", "track", ses.name(), "from", int(expected), "to", int(sm.Position))
	case sm.Position >= StartZone:
		s.Log.Debug("прыжок вперёд", "track", ses.name(), "from", int(expected), "to", int(sm.Position))
		ses.jumps = append(ses.jumps, sm.Position)
	}
	ses.pos, ses.at, ses.playing = sm.Position, now, sm.Playing
}

func newSession(key string, sm Sample, now time.Time) *session {
	return &session{
		key: key, artist: strings.TrimSpace(sm.Artist), title: strings.TrimSpace(sm.Title), length: sm.Length,
		pos: sm.Position, at: now, playing: sm.Playing, counts: make([]uint8, int(math.Ceil(sm.Length))+1),
	}
}

func (ses *session) mark(from, to float64) {
	for i := int(math.Ceil(from)); float64(i) < to && i < len(ses.counts); i++ {
		if i >= 0 && ses.counts[i] < math.MaxUint8 {
			ses.counts[i]++
		}
	}
}

func (ses *session) name() string { return ses.artist + " — " + ses.title }

func (ses *session) heat() map[int]float64 {
	heat := map[int]float64{}
	for i, c := range ses.counts {
		if c >= 2 && float64(i) >= StartZone {
			heat[i] += float64(c - 1)
		}
	}
	for _, p := range ses.jumps {
		for i := int(p); i < int(p)+jumpSpan && float64(i) < ses.length; i++ {
			heat[i] += jumpWeight
		}
	}
	return heat
}

func (s *Store) finish(ses *session, now time.Time) {
	heat := ses.heat()
	if len(heat) == 0 {
		return
	}
	s.Log.Info("переслушивания учтены для любимого момента", "track", ses.name(), "seconds", len(heat))
	t := s.tracks[ses.key]
	if t == nil {
		t = &track{Artist: ses.artist, Title: ses.title}
		s.tracks[ses.key] = t
	}
	if t.Heat == nil {
		t.Heat = map[int]float64{}
	}
	for i, v := range heat {
		t.Heat[i] += v
	}
	t.Length, t.Seen = ses.length, now.UTC()
	s.saveLocked()
}

func (s *Store) Close(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, ses := range s.sessions {
		s.finish(ses, now)
		delete(s.sessions, id)
	}
}

func (s *Store) Moment(artist, title string, length float64) (Moment, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := Key(artist, title)
	t := s.tracks[key]
	if t != nil && length <= 0 {
		length = t.Length
	}
	if t != nil && t.Manual != nil {
		end := t.Manual.End
		if length > 0 {
			end = math.Min(end, length)
		}
		if end > t.Manual.Start {
			return Moment{Start: t.Manual.Start, End: end, Manual: true}, true
		}
	}
	heat := map[int]float64{}
	if t != nil {
		for i, v := range t.Heat {
			heat[i] = v
		}
	}
	for _, ses := range s.sessions {
		if ses.key != key {
			continue
		}
		if length <= 0 {
			length = ses.length
		}
		for i, v := range ses.heat() {
			heat[i] += v
		}
	}
	return best(heat, length)
}

func best(heat map[int]float64, length float64) (Moment, bool) {
	n := int(length)
	lo := int(StartZone)
	if len(heat) == 0 || n < lo+window {
		return Moment{}, false
	}
	arr := make([]float64, n)
	for i, v := range heat {
		if i >= 0 && i < n {
			arr[i] = v
		}
	}
	sum := 0.0
	for i := lo; i < lo+window; i++ {
		sum += arr[i]
	}
	bestSum, bestAt := sum, lo
	for i := lo + 1; i+window <= n; i++ {
		sum += arr[i+window-1] - arr[i-1]
		if sum > bestSum+1e-9 {
			bestSum, bestAt = sum, i
		}
	}
	if bestSum < minHeat {
		return Moment{}, false
	}
	peak := bestSum / window
	start, end := bestAt, bestAt+window
	for start > lo && arr[start-1] >= peak/2 && end-start < maxMoment {
		start--
	}
	for end < n && arr[end] >= peak/2 && end-start < maxMoment {
		end++
	}
	for end-start > minMoment && arr[start] < peak/4 {
		start++
	}
	for end-start > minMoment && arr[end-1] < peak/4 {
		end--
	}
	return Moment{Start: float64(start), End: float64(end)}, true
}

func (s *Store) SetManual(artist, title string, length float64, m Moment, now time.Time) error {
	if norm(artist) == "" || norm(title) == "" {
		return errors.New("неизвестный трек")
	}
	if length <= 0 || length > maxLength {
		return errors.New("у трека неизвестна длина")
	}
	m.End = math.Min(m.End, length)
	if m.Start < 0 || m.Start >= length || m.End-m.Start < 1 {
		return errors.New("момент должен быть внутри трека и длиться хотя бы секунду")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := Key(artist, title)
	t := s.tracks[key]
	if t == nil {
		t = &track{Artist: strings.TrimSpace(artist), Title: strings.TrimSpace(title)}
		s.tracks[key] = t
	}
	t.Length, t.Seen = length, now.UTC()
	t.Manual = &Moment{Start: math.Round(m.Start*10) / 10, End: math.Round(m.End*10) / 10, Manual: true}
	return s.saveLocked()
}

func (s *Store) ClearManual(artist, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := Key(artist, title)
	t := s.tracks[key]
	if t == nil || t.Manual == nil {
		return nil
	}
	t.Manual = nil
	if len(t.Heat) == 0 {
		delete(s.tracks, key)
	}
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	if len(s.tracks) > maxTracks {
		keys := make([]string, 0, len(s.tracks))
		for k := range s.tracks {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return s.tracks[keys[i]].Seen.Before(s.tracks[keys[j]].Seen) })
		for _, k := range keys[:len(keys)-maxTracks] {
			delete(s.tracks, k)
		}
	}
	data, err := json.Marshal(stored{Version: fileVer, Tracks: s.tracks})
	if err == nil {
		err = atomicfile.Write(s.path, data)
	}
	if err != nil {
		s.Log.Warn("не удалось сохранить любимые моменты", "err", err)
	}
	return err
}
