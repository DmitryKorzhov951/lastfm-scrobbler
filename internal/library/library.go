package library

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/atomicfile"
)

type Play struct {
	At     time.Time
	Artist string
	Title  string
	Album  string
}

type Meta struct {
	User       string `json:"user"`
	Complete   bool   `json:"complete"`
	ImportTo   int64  `json:"import_to,omitempty"`
	NextPage   int    `json:"next_page,omitempty"`
	TotalPages int    `json:"total_pages,omitempty"`
	Total      int    `json:"total,omitempty"`
	SyncedAt   int64  `json:"synced_at,omitempty"`
}

type rec struct {
	ts     int64
	artist int32
	track  int32
	album  int32
}

type name struct {
	text   string
	last   int64
	artist int32
}

type table struct {
	idx  map[string]int32
	list []name
}

func newTable() table { return table{idx: map[string]int32{}} }

func (t *table) get(key, text string, ts int64, artist int32) int32 {
	if id, ok := t.idx[key]; ok {
		if ts >= t.list[id].last {
			t.list[id].text, t.list[id].last = text, ts
		}
		return id
	}
	id := int32(len(t.list))
	t.idx[key] = id
	t.list = append(t.list, name{text: text, last: ts, artist: artist})
	return id
}

func (t *table) find(key string) (int32, bool) {
	id, ok := t.idx[key]
	return id, ok
}

type Store struct {
	path     string
	metaPath string

	mu      sync.Mutex
	meta    Meta
	plays   []rec
	sorted  bool
	seen    map[uint64]struct{}
	artists table
	tracks  table
	albums  table
}

func Open(path, metaPath string) (*Store, error) {
	s := &Store{path: path, metaPath: metaPath}
	s.clear()
	if path == "" {
		return s, nil
	}
	if data, err := os.ReadFile(metaPath); err == nil {
		if err := json.Unmarshal(data, &s.meta); err != nil {
			s.meta = Meta{}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	bad, err := s.load()
	if err != nil {
		return nil, err
	}
	if bad {
		if err := s.rewriteLocked(); err != nil {
			return s, fmt.Errorf("история прослушиваний: повреждённые строки убраны не до конца: %w", err)
		}
	}
	return s, nil
}

func (s *Store) clear() {
	s.plays = nil
	s.sorted = true
	s.seen = map[uint64]struct{}{}
	s.artists, s.tracks, s.albums = newTable(), newTable(), newTable()
}

func (s *Store) load() (bad bool, err error) {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<16)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			if p, ok := decodeLine(line); ok {
				s.addLocked(p)
			} else if strings.TrimSpace(string(line)) != "" {
				bad = true
			}
		}
		if err == io.EOF {
			return bad, nil
		}
		if err != nil {
			return bad, err
		}
	}
}

func decodeLine(line []byte) (Play, bool) {
	b := bytes.TrimSpace(line)
	if len(b) < 2 || b[0] != '[' || b[len(b)-1] != ']' {
		return Play{}, false
	}
	b = b[1 : len(b)-1]
	i := bytes.IndexByte(b, ',')
	if i < 0 {
		return Play{}, false
	}
	ts, err := strconv.ParseInt(string(bytes.TrimSpace(b[:i])), 10, 64)
	if err != nil {
		return Play{}, false
	}
	rest := b[i+1:]
	var out [3]string
	n := 0
	for n < 3 {
		rest = bytes.TrimLeft(rest, " ")
		if len(rest) == 0 {
			break
		}
		if rest[0] != '"' {
			return Play{}, false
		}
		j, esc := 1, false
		for ; j < len(rest); j++ {
			c := rest[j]
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				break
			}
		}
		if j >= len(rest) {
			return Play{}, false
		}
		v, ok := unquote(rest[:j+1])
		if !ok {
			return Play{}, false
		}
		out[n] = v
		n++
		rest = bytes.TrimLeft(rest[j+1:], " ")
		if len(rest) > 0 {
			if rest[0] != ',' {
				return Play{}, false
			}
			rest = rest[1:]
		}
	}
	if n < 2 || len(bytes.TrimSpace(rest)) > 0 {
		return Play{}, false
	}
	return Play{At: time.Unix(ts, 0), Artist: out[0], Title: out[1], Album: out[2]}, true
}

func unquote(b []byte) (string, bool) {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return "", false
	}
	inner := b[1 : len(b)-1]
	if bytes.IndexByte(inner, '\\') < 0 {
		return string(inner), true
	}
	var s string
	if json.Unmarshal(b, &s) != nil {
		return "", false
	}
	return s, true
}

func encodeLine(p Play) []byte {
	b, _ := json.Marshal([]any{p.At.Unix(), p.Artist, p.Title, p.Album})
	return append(b, '\n')
}

func norm(s string) string {
	clean := true
	prevSpace := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x80 || ('A' <= c && c <= 'Z') || c == '\t' || c == '\n' || c == '\r' || (c == ' ' && prevSpace) {
			clean = false
			break
		}
		prevSpace = c == ' '
	}
	if clean && !prevSpace {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = b.Len() > 0
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

type keys struct {
	artist, track, album string
}

func keysOf(p Play) keys {
	a := norm(p.Artist)
	k := keys{artist: a, track: a + "\x00" + norm(p.Title)}
	if al := norm(p.Album); al != "" {
		k.album = a + "\x00" + al
	}
	return k
}

func dedupeKey(ts int64, track string) uint64 {
	h := fnv.New64a()
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(ts))
	h.Write(b[:])
	h.Write([]byte(track))
	return h.Sum64()
}

func (s *Store) intern(p Play, k keys) rec {
	ts := p.At.Unix()
	a := s.artists.get(k.artist, strings.TrimSpace(p.Artist), ts, -1)
	t := s.tracks.get(k.track, strings.TrimSpace(p.Title), ts, a)
	al := int32(-1)
	if k.album != "" {
		al = s.albums.get(k.album, strings.TrimSpace(p.Album), ts, a)
	}
	return rec{ts: ts, artist: a, track: t, album: al}
}

func valid(p Play) bool {
	return !p.At.IsZero() && strings.TrimSpace(p.Artist) != "" && strings.TrimSpace(p.Title) != ""
}

func (s *Store) addLocked(p Play) bool {
	if !valid(p) {
		return false
	}
	k := keysOf(p)
	d := dedupeKey(p.At.Unix(), k.track)
	if _, dup := s.seen[d]; dup {
		return false
	}
	s.seen[d] = struct{}{}
	r := s.intern(p, k)
	if n := len(s.plays); n > 0 && s.plays[n-1].ts > r.ts {
		s.sorted = false
	}
	s.plays = append(s.plays, r)
	return true
}

func (s *Store) Add(plays []Play) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var buf []byte
	added := 0
	for _, p := range plays {
		if s.addLocked(p) {
			added++
			buf = append(buf, encodeLine(p)...)
		}
	}
	if added == 0 || s.path == "" {
		return added, nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return added, err
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return added, err
	}
	if _, err := f.Write(buf); err != nil {
		f.Close()
		return added, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return added, err
	}
	return added, f.Close()
}

func (s *Store) Remove(plays []Play) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	drop := make(map[uint64]bool, len(plays))
	for _, p := range plays {
		if valid(p) {
			drop[dedupeKey(p.At.Unix(), keysOf(p).track)] = true
		}
	}
	kept := s.plays[:0]
	n := 0
	for _, r := range s.plays {
		if d := dedupeKey(r.ts, keysOf(s.play(r)).track); drop[d] {
			delete(s.seen, d)
			n++
			continue
		}
		kept = append(kept, r)
	}
	s.plays = kept
	if n == 0 || s.path == "" {
		return n, nil
	}
	return n, s.rewriteLocked()
}

func (s *Store) rewriteLocked() error {
	s.sortLocked()
	var buf []byte
	for _, r := range s.plays {
		buf = append(buf, encodeLine(s.play(r))...)
	}
	return atomicfile.Write(s.path, buf)
}

func (s *Store) play(r rec) Play {
	p := Play{At: time.Unix(r.ts, 0), Artist: s.artists.list[r.artist].text, Title: s.tracks.list[r.track].text}
	if r.album >= 0 {
		p.Album = s.albums.list[r.album].text
	}
	return p
}

func (s *Store) Reset(user string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clear()
	s.meta = Meta{User: user}
	if s.path == "" {
		return nil
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.saveMetaLocked()
}

func (s *Store) Meta() Meta {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.meta
}

func (s *Store) SaveMeta(m Meta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meta = m
	return s.saveMetaLocked()
}

func (s *Store) saveMetaLocked() error {
	if s.metaPath == "" {
		return nil
	}
	data, err := json.MarshalIndent(s.meta, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(s.metaPath, data)
}

func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.plays)
}

func (s *Store) Newest() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sortLocked()
	if len(s.plays) == 0 {
		return time.Time{}
	}
	return time.Unix(s.plays[len(s.plays)-1].ts, 0)
}

func (s *Store) Recent(n int) []Play {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sortLocked()
	n = min(n, len(s.plays))
	out := make([]Play, 0, max(n, 0))
	for i := len(s.plays) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, s.play(s.plays[i]))
	}
	return out
}

func (s *Store) sortLocked() {
	if s.sorted {
		return
	}
	sort.SliceStable(s.plays, func(i, j int) bool { return s.plays[i].ts < s.plays[j].ts })
	s.sorted = true
}

func (s *Store) hasTs(ts int64) bool {
	i := sort.Search(len(s.plays), func(i int) bool { return s.plays[i].ts >= ts })
	return i < len(s.plays) && s.plays[i].ts == ts
}

type view struct {
	s     *Store
	extra []rec
}

func (s *Store) viewLocked(extra []Play) view {
	s.sortLocked()
	v := view{s: s}
	for _, p := range extra {
		if !valid(p) || s.hasTs(p.At.Unix()) {
			continue
		}
		v.extra = append(v.extra, s.intern(p, keysOf(p)))
	}
	return v
}

func (v view) each(from, to int64, fn func(rec)) {
	plays := v.s.plays
	i := sort.Search(len(plays), func(i int) bool { return plays[i].ts >= from })
	for ; i < len(plays) && plays[i].ts < to; i++ {
		fn(plays[i])
	}
	for _, r := range v.extra {
		if r.ts >= from && r.ts < to {
			fn(r)
		}
	}
}

func (v view) first() (int64, bool) {
	best, ok := int64(0), false
	if len(v.s.plays) > 0 {
		best, ok = v.s.plays[0].ts, true
	}
	for _, r := range v.extra {
		if !ok || r.ts < best {
			best, ok = r.ts, true
		}
	}
	return best, ok
}

const (
	never  = int64(-1 << 62)
	always = int64(1 << 62)
)
