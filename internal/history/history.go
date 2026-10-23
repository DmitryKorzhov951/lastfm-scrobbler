package history

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/atomicfile"
)

const DefaultMax = 2000

type Status string

const (
	Queued  Status = "queued"
	Sent    Status = "sent"
	Ignored Status = "ignored"
)

type Entry struct {
	ID      int64     `json:"id"`
	Artist  string    `json:"artist"`
	Title   string    `json:"title"`
	Album   string    `json:"album,omitempty"`
	Seconds int       `json:"seconds,omitempty"`
	Player  string    `json:"player,omitempty"`
	Started time.Time `json:"started"`
	Status  Status    `json:"status"`
	Reason  string    `json:"reason,omitempty"`
}

func (e Entry) counted() bool { return e.Status != Ignored }

type Player struct {
	Name     string `json:"name"`
	Identity string `json:"identity"`
}

type file struct {
	Entries      []Entry  `json:"entries"`
	Total        int      `json:"total"`
	TotalSeconds int64    `json:"total_seconds"`
	LastID       int64    `json:"last_id"`
	Players      []Player `json:"players,omitempty"`
}

type Store struct {
	path string
	max  int
	mu   sync.Mutex
	f    file
}

func Open(path string, max int) (*Store, error) {
	if max <= 0 {
		max = DefaultMax
	}
	s := &Store{path: path, max: max}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if len(data) > 0 {
			if err := json.Unmarshal(data, &s.f); err != nil {
				broken := path + ".broken"
				_ = os.Rename(path, broken)
				s.f = file{}
				return s, fmt.Errorf("история повреждена, файл сохранён как %s: %w", broken, err)
			}
		}
	case os.IsNotExist(err):
	default:
		return nil, fmt.Errorf("чтение истории %s: %w", path, err)
	}
	return s, nil
}

func (s *Store) LastID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f.LastID
}

func (s *Store) Add(e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.Status == "" {
		e.Status = Queued
	}
	s.f.Entries = append(s.f.Entries, e)
	if len(s.f.Entries) > s.max {
		s.f.Entries = append([]Entry(nil), s.f.Entries[len(s.f.Entries)-s.max:]...)
	}
	if e.counted() {
		s.f.Total++
		s.f.TotalSeconds += int64(e.Seconds)
	}
	if e.ID > s.f.LastID {
		s.f.LastID = e.ID
	}
	return s.saveLocked()
}

func (s *Store) SetStatus(st Status, reason string, ids ...int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	want := idSet(ids)
	changed := false
	for i := range s.f.Entries {
		e := &s.f.Entries[i]
		if !want[e.ID] || e.Status == st {
			continue
		}
		wasCounted := e.counted()
		e.Status = st
		if st == Ignored {
			e.Reason = reason
		}
		s.adjustLocked(*e, wasCounted)
		changed = true
	}
	if !changed {
		return nil
	}
	return s.saveLocked()
}

func (s *Store) Delete(ids ...int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	drop := idSet(ids)
	kept := make([]Entry, 0, len(s.f.Entries))
	for _, e := range s.f.Entries {
		if !drop[e.ID] {
			kept = append(kept, e)
			continue
		}
		if e.counted() {
			s.f.Total--
			s.f.TotalSeconds -= int64(e.Seconds)
		}
	}
	if len(kept) == len(s.f.Entries) {
		return nil
	}
	s.f.Entries = kept
	s.clampLocked()
	return s.saveLocked()
}

func (s *Store) Recent(n int) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n > len(s.f.Entries) {
		n = len(s.f.Entries)
	}
	out := make([]Entry, 0, n)
	for i := len(s.f.Entries) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, s.f.Entries[i])
	}
	return out
}

type Totals struct {
	TodayCount   int
	TodayTime    time.Duration
	AllCount     int
	AllTime      time.Duration
	TodayStarted time.Time
}

func (s *Store) Totals(now time.Time) Totals {
	s.mu.Lock()
	defer s.mu.Unlock()
	y, m, d := now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	t := Totals{
		AllCount:     s.f.Total,
		AllTime:      time.Duration(s.f.TotalSeconds) * time.Second,
		TodayStarted: midnight,
	}
	for _, e := range s.f.Entries {
		if e.counted() && !e.Started.Before(midnight) {
			t.TodayCount++
			t.TodayTime += time.Duration(e.Seconds) * time.Second
		}
	}
	return t
}

func (s *Store) SeePlayer(name, identity string) error {
	if name == "" {
		return nil
	}
	if identity == "" {
		identity = name
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, p := range s.f.Players {
		if p.Name == name {
			if p.Identity == identity {
				return nil
			}
			s.f.Players[i].Identity = identity
			return s.saveLocked()
		}
	}
	s.f.Players = append(s.f.Players, Player{Name: name, Identity: identity})
	sort.Slice(s.f.Players, func(i, j int) bool { return s.f.Players[i].Name < s.f.Players[j].Name })
	return s.saveLocked()
}

func (s *Store) Players() []Player {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Player(nil), s.f.Players...)
}

func (s *Store) adjustLocked(e Entry, wasCounted bool) {
	switch {
	case wasCounted && !e.counted():
		s.f.Total--
		s.f.TotalSeconds -= int64(e.Seconds)
	case !wasCounted && e.counted():
		s.f.Total++
		s.f.TotalSeconds += int64(e.Seconds)
	}
	s.clampLocked()
}

func (s *Store) clampLocked() {
	if s.f.Total < 0 {
		s.f.Total = 0
	}
	if s.f.TotalSeconds < 0 {
		s.f.TotalSeconds = 0
	}
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	data, err := json.Marshal(s.f)
	if err != nil {
		return err
	}
	return atomicfile.Write(s.path, data)
}

func idSet(ids []int64) map[int64]bool {
	m := make(map[int64]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}
