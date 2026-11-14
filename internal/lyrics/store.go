package lyrics

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/atomicfile"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

const (
	Loading = "loading"
	Found   = "found"
	Missing = "missing"
	Failed  = "error"

	retryFailed     = 2 * time.Minute
	retryIncomplete = 30 * time.Minute
	retryMissing    = 12 * time.Hour
	recheckGap      = 5 * time.Second
	lookupLimit     = 45 * time.Second
	maxKnown        = 300
	diskLimit       = 1000
	maxFile         = 1 << 20
)

type Finder func(ctx context.Context, q Query) (Lyrics, error)

type Result struct {
	State string
	Lyrics
}

type Store struct {
	Log    *slog.Logger
	Now    func() time.Time
	Folder func() string

	dir  string
	find Finder

	mu    sync.Mutex
	known map[string]*entry
	busy  sync.WaitGroup
}

type entry struct {
	state   string
	lyrics  Lyrics
	at      time.Time
	wait    time.Duration
	checked time.Time
}

type cached struct {
	Artist string    `json:"artist"`
	Title  string    `json:"title"`
	Album  string    `json:"album,omitempty"`
	Saved  time.Time `json:"saved"`
	Lyrics Lyrics    `json:"lyrics"`
}

func New(dir string, find Finder, log *slog.Logger) *Store {
	if log == nil {
		log = slog.Default()
	}
	return &Store{Log: log, Now: time.Now, dir: dir, find: find, known: map[string]*entry{}}
}

func (s *Store) Get(t media.Track) Result {
	if !t.Valid() {
		return Result{State: Missing}
	}
	key := lookupKey(t)
	if l, ok := s.recheck(key, t); ok {
		return Result{State: Found, Lyrics: l}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.known[key]; e != nil {
		switch {
		case e.state == Loading:
			return Result{State: Loading}
		case e.state == Found:
			return Result{State: Found, Lyrics: e.lyrics}
		case s.Now().Before(e.at.Add(e.wait)):
			return Result{State: e.state}
		}
	}
	if len(s.known) >= maxKnown {
		for k, e := range s.known {
			if e.state != Loading {
				delete(s.known, k)
			}
		}
	}
	s.known[key] = &entry{state: Loading}
	s.busy.Add(1)
	go s.resolve(key, t)
	return Result{State: Loading}
}

func (s *Store) Wait() { s.busy.Wait() }

func (s *Store) recheck(key string, t media.Track) (Lyrics, bool) {
	s.mu.Lock()
	e := s.known[key]
	due := e != nil && e.state == Missing && !s.Now().Before(e.checked.Add(recheckGap))
	if due {
		e.checked = s.Now()
	}
	s.mu.Unlock()
	if !due {
		return Lyrics{}, false
	}
	l := s.files(t)
	if l.Empty() {
		return Lyrics{}, false
	}
	s.mu.Lock()
	if s.known[key] == e && e.state == Missing {
		e.state, e.lyrics = Found, l
	}
	s.mu.Unlock()
	return l, true
}

func (s *Store) resolve(key string, t media.Track) {
	defer s.busy.Done()
	l, err := s.lookup(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.known[key]
	if e == nil {
		e = &entry{}
		s.known[key] = e
	}
	e.at = s.Now()
	e.checked = e.at
	var part *Incomplete
	switch {
	case err == nil && !l.Empty():
		e.state, e.lyrics = Found, l
	case errors.As(err, &part):
		e.state, e.wait = Missing, retryIncomplete
		s.Log.Debug("текст песни не найден, но не все сайты ответили", "track", t.String(), "err", part.Err)
	case err == nil || errors.Is(err, ErrNotFound):
		e.state, e.wait = Missing, retryMissing
		s.Log.Debug("текст песни не найден", "track", t.String())
	default:
		e.state, e.wait = Failed, retryFailed
		s.Log.Debug("не удалось получить текст песни", "track", t.String(), "err", err)
	}
}

func (s *Store) files(t media.Track) Lyrics {
	bases := names(t.Artist(), t.Title)
	if path := media.LocalPath(t.URL); path != "" {
		if l := nearSong(path, bases); !l.Empty() {
			return l
		}
	}
	if s.Folder != nil {
		if dir := s.Folder(); dir != "" {
			return findNamed(dir, bases, FromFolder)
		}
	}
	return Lyrics{}
}

func (s *Store) local(t media.Track) Lyrics {
	if l := s.files(t); !l.Empty() {
		return l
	}
	if path := media.LocalPath(t.URL); path != "" {
		return fromTags(path)
	}
	return Lyrics{}
}

func (s *Store) lookup(t media.Track) (Lyrics, error) {
	if l := s.local(t); !l.Empty() {
		return l, nil
	}
	cache := s.cachePath(t)
	if l, ok := readCache(cache, s.Now()); ok {
		return l, nil
	}
	if s.find == nil {
		return Lyrics{}, ErrNotFound
	}
	ctx, cancel := context.WithTimeout(context.Background(), lookupLimit)
	defer cancel()
	l, err := s.find(ctx, Query{Artist: t.Artist(), Title: t.Title, Album: t.Album, Duration: t.Length.Seconds()})
	if err != nil {
		return Lyrics{}, err
	}
	if !l.Empty() && cache != "" {
		s.writeCache(cache, t, l)
	}
	return l, nil
}

func nearSong(path string, bases []string) Lyrics {
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Lyrics{}
	}
	own := append([]string{strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))}, bases...)
	if l := pickNamed(dir, entries, own, FromFile); !l.Empty() {
		return l
	}
	for _, e := range entries {
		if e.IsDir() && strings.EqualFold(e.Name(), "lyrics") {
			if l := findNamed(filepath.Join(dir, e.Name()), own, FromFile); !l.Empty() {
				return l
			}
		}
	}
	return Lyrics{}
}

func findNamed(dir string, bases []string, source string) Lyrics {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Lyrics{}
	}
	return pickNamed(dir, entries, bases, source)
}

func pickNamed(dir string, entries []os.DirEntry, bases []string, source string) Lyrics {
	files := map[string]string{}
	for _, e := range entries {
		name := norm(e.Name())
		if ext := filepath.Ext(name); !e.IsDir() && (ext == ".lrc" || ext == ".txt") {
			files[name] = e.Name()
		}
	}
	if len(files) == 0 {
		return Lyrics{}
	}
	for _, b := range bases {
		for _, ext := range []string{".lrc", ".txt"} {
			name, ok := files[norm(b+ext)]
			if !ok {
				continue
			}
			if data, err := readLimited(filepath.Join(dir, name)); err == nil {
				if l := FromText(decode(data), source); !l.Empty() {
					return l
				}
			}
		}
	}
	return Lyrics{}
}

func names(artist, title string) []string {
	var out []string
	seen := map[string]bool{}
	for _, a := range []string{artist, mainArtist(artist)} {
		for _, t := range []string{title, CleanTitle(title)} {
			for _, keep := range []bool{true, false} {
				sa, st := safeName(a, keep), safeName(t, keep)
				if sa == "" || st == "" || seen[norm(sa+" - "+st)] {
					continue
				}
				seen[norm(sa+" - "+st)] = true
				out = append(out, sa+" - "+st)
			}
		}
	}
	return out
}

func safeName(s string, underscore bool) string {
	var b strings.Builder
	for _, r := range strings.Join(strings.Fields(s), " ") {
		if r < 32 || strings.ContainsRune(`\/:*?"<>|`, r) {
			if underscore {
				b.WriteRune('_')
			}
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func fromTags(path string) (l Lyrics) {
	f, err := os.Open(path)
	if err != nil {
		return Lyrics{}
	}
	defer f.Close()
	defer func() {
		if recover() != nil {
			l = Lyrics{}
		}
	}()
	m, err := tag.ReadFrom(f)
	if err != nil {
		return Lyrics{}
	}
	text := m.Lyrics()
	if strings.TrimSpace(text) == "" {
		text = extraLyrics(m.Raw())
	}
	return FromText(text, FromTags)
}

func extraLyrics(raw map[string]interface{}) string {
	for _, k := range []string{"unsyncedlyrics", "unsynced lyrics", "lyrics"} {
		if v, ok := raw[k].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		if strings.HasPrefix(k, "TXX") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		c, ok := raw[k].(*tag.Comm)
		if !ok {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(c.Description)) {
		case "LYRICS", "UNSYNCEDLYRICS", "UNSYNCED LYRICS":
			if strings.TrimSpace(c.Text) != "" {
				return c.Text
			}
		}
	}
	return ""
}

func readLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFile+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFile {
		return nil, errors.New("слишком большой файл")
	}
	return data, nil
}

func (s *Store) cachePath(t media.Track) string {
	if s.dir == "" {
		return ""
	}
	sum := sha1.Sum([]byte(norm(t.Artist()) + "\x00" + norm(t.Title) + "\x00" + norm(t.Album)))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}

func readCache(path string, now time.Time) (Lyrics, bool) {
	if path == "" {
		return Lyrics{}, false
	}
	data, err := readLimited(path)
	if err != nil {
		return Lyrics{}, false
	}
	var c cached
	if json.Unmarshal(data, &c) != nil || c.Lyrics.Empty() {
		return Lyrics{}, false
	}
	os.Chtimes(path, now, now)
	return c.Lyrics, true
}

func (s *Store) writeCache(path string, t media.Track, l Lyrics) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		s.Log.Debug("не удалось создать папку текстов песен", "err", err)
		return
	}
	data, err := json.Marshal(cached{Artist: t.Artist(), Title: t.Title, Album: t.Album, Saved: s.Now().UTC(), Lyrics: l})
	if err != nil {
		return
	}
	if err := atomicfile.Write(path, data); err != nil {
		s.Log.Debug("не удалось сохранить текст песни", "err", err)
		return
	}
	s.prune()
}

func (s *Store) prune() {
	entries, err := os.ReadDir(s.dir)
	if err != nil || len(entries) <= diskLimit {
		return
	}
	type file struct {
		name string
		mod  time.Time
	}
	files := make([]file, 0, len(entries))
	for _, e := range entries {
		if info, err := e.Info(); err == nil && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, file{e.Name(), info.ModTime()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	for i := 0; i < len(files)-diskLimit*9/10; i++ {
		os.Remove(filepath.Join(s.dir, files[i].name))
	}
}

func lookupKey(t media.Track) string {
	return strings.Join([]string{norm(t.Artist()), norm(t.Title), norm(t.Album), media.LocalPath(t.URL)}, "\x00")
}

func norm(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
