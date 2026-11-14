package cover

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/atomicfile"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

const (
	MaxSize = 8 << 20

	memImages    = 48
	maxKnown     = 400
	diskLimit    = 500
	retryFailed  = 2 * time.Minute
	retryMissing = 6 * time.Hour
	lookupLimit  = 45 * time.Second
)

type Image struct {
	Data []byte
	Type string
}

type Finder func(ctx context.Context, artist, title, album string) (string, error)

type Store struct {
	Log       *slog.Logger
	HTTP      *http.Client
	UserAgent string
	Now       func() time.Time

	dir string

	mu      sync.Mutex
	find    Finder
	players map[string]source
	known   map[string]*result
	images  map[string]Image
	order   []string
	busy    sync.WaitGroup
}

type source struct {
	track  string
	art    []byte
	artKey string
	file   string
}

type result struct {
	key     string
	pending bool
	failed  time.Time
	wait    time.Duration
}

func New(dir string, log *slog.Logger) *Store {
	if log == nil {
		log = slog.Default()
	}
	return &Store{
		Log:     log,
		Now:     time.Now,
		dir:     dir,
		players: map[string]source{},
		known:   map[string]*result{},
		images:  map[string]Image{},
	}
}

func (s *Store) SetFinder(f Finder) {
	s.mu.Lock()
	s.find = f
	for k, r := range s.known {
		if !r.pending && r.key == "" {
			delete(s.known, k)
		}
	}
	s.mu.Unlock()
}

func (s *Store) Note(st media.State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st.Gone {
		delete(s.players, st.ID)
		return
	}
	src := source{track: st.Track.Key(), file: media.LocalPath(st.Track.URL)}
	if len(st.Art) > 0 && len(st.Art) <= MaxSize && Sniff(st.Art) != "" {
		if prev := s.players[st.ID]; prev.artKey != "" && string(prev.art) == string(st.Art) {
			src.art, src.artKey = prev.art, prev.artKey
		} else {
			src.art, src.artKey = st.Art, contentKey(st.Art)
		}
	}
	s.players[st.ID] = src
}

func (s *Store) Key(id string, t media.Track) string {
	if !t.Valid() {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	src, ok := s.players[id]
	if !ok || src.track != t.Key() {
		src = source{}
	}
	if src.artKey != "" {
		s.keepLocked(src.artKey, src.art)
		return src.artKey
	}
	lk := lookupID(t, src)
	if r := s.known[lk]; r != nil {
		switch {
		case r.pending:
			return ""
		case r.key != "":
			if _, ok := s.images[r.key]; ok {
				s.touchLocked(r.key)
				return r.key
			}
		case s.Now().Before(r.failed.Add(r.wait)):
			return ""
		}
	}
	if len(s.known) >= maxKnown {
		for k, r := range s.known {
			if !r.pending {
				delete(s.known, k)
			}
		}
	}
	s.known[lk] = &result{pending: true}
	s.busy.Add(1)
	go s.resolve(lk, t, src, s.find)
	return ""
}

func (s *Store) Image(key string) (Image, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	img, ok := s.images[key]
	return img, ok
}

func (s *Store) Wait() { s.busy.Wait() }

func (s *Store) resolve(id string, t media.Track, src source, find Finder) {
	defer s.busy.Done()
	ctx, cancel := context.WithTimeout(context.Background(), lookupLimit)
	defer cancel()
	data, missing, err := s.lookup(ctx, t, src, find)

	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.known[id]
	if r == nil {
		r = &result{}
		s.known[id] = r
	}
	r.pending = false
	switch {
	case err == nil && len(data) > 0:
		r.key = contentKey(data)
		s.keepLocked(r.key, data)
	case missing:
		r.failed, r.wait = s.Now(), retryMissing
		s.Log.Debug("обложка не найдена", "track", t.String())
	default:
		r.failed, r.wait = s.Now(), retryFailed
		s.Log.Debug("не удалось получить обложку", "track", t.String(), "err", err)
	}
}

func (s *Store) lookup(ctx context.Context, t media.Track, src source, find Finder) ([]byte, bool, error) {
	if src.file != "" {
		if data := FromFile(src.file); data != nil {
			return data, false, nil
		}
	}
	name := s.cachePath(t)
	if data := readImage(name); data != nil {
		return data, false, nil
	}
	if find == nil {
		return nil, true, nil
	}
	u, err := find(ctx, t.Artist(), t.Title, t.Album)
	if err != nil {
		return nil, false, err
	}
	if u == "" {
		return nil, true, nil
	}
	data, err := s.download(ctx, u)
	if err != nil {
		return nil, false, err
	}
	if name != "" {
		if err := os.MkdirAll(filepath.Dir(name), 0o700); err == nil {
			if err := atomicfile.Write(name, data); err != nil {
				s.Log.Debug("не удалось сохранить обложку", "err", err)
			}
			s.prune()
		}
	}
	return data, false, nil
}

func (s *Store) download(ctx context.Context, raw string) ([]byte, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("неподдерживаемый адрес обложки %q", raw)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if s.UserAgent != "" {
		req.Header.Set("User-Agent", s.UserAgent)
	}
	client := s.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("сервер обложек ответил %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxSize || Sniff(data) == "" {
		return nil, errors.New("по адресу обложки не картинка")
	}
	return data, nil
}

func (s *Store) keepLocked(key string, data []byte) {
	if _, ok := s.images[key]; ok {
		s.touchLocked(key)
		return
	}
	s.images[key] = Image{Data: data, Type: Sniff(data)}
	s.order = append(s.order, key)
	for len(s.order) > memImages {
		delete(s.images, s.order[0])
		s.order = s.order[1:]
	}
}

func (s *Store) touchLocked(key string) {
	for i, k := range s.order {
		if k == key {
			copy(s.order[i:], s.order[i+1:])
			s.order[len(s.order)-1] = key
			return
		}
	}
}

func (s *Store) cachePath(t media.Track) string {
	if s.dir == "" {
		return ""
	}
	id := norm(t.Artist()) + "\x00"
	if t.Album != "" {
		id += "album\x00" + norm(t.Album)
	} else {
		id += "track\x00" + norm(t.Title)
	}
	sum := sha1.Sum([]byte(id))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".img")
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
		if info, err := e.Info(); err == nil && strings.HasSuffix(e.Name(), ".img") {
			files = append(files, file{e.Name(), info.ModTime()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	for i := 0; i < len(files)-diskLimit*9/10; i++ {
		os.Remove(filepath.Join(s.dir, files[i].name))
	}
}

func lookupID(t media.Track, src source) string {
	return strings.Join([]string{norm(t.Artist()), norm(t.Title), norm(t.Album), src.file}, "\x00")
}

func contentKey(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:10])
}

func Sniff(data []byte) string {
	if len(data) < 8 {
		return ""
	}
	ct := http.DetectContentType(data)
	switch ct {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp":
		return ct
	}
	return ""
}

func readImage(path string) []byte {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxSize+1))
	if err != nil || len(data) > MaxSize || Sniff(data) == "" {
		return nil
	}
	return data
}

func norm(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
