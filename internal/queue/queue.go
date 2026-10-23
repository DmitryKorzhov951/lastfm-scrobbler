package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/atomicfile"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lastfm"
)

const DefaultMax = 5000

type Item struct {
	ID       int64           `json:"id,omitempty"`
	Scrobble lastfm.Scrobble `json:"scrobble"`
	QueuedAt time.Time       `json:"queued_at"`
	Attempts int             `json:"attempts"`
}

type Queue struct {
	path   string
	max    int
	mu     sync.Mutex
	items  []Item
	lastID int64
}

func Open(path string, max int) (*Queue, error) {
	if max <= 0 {
		max = DefaultMax
	}
	q := &Queue{path: path, max: max}

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if len(data) > 0 {
			if err := json.Unmarshal(data, &q.items); err != nil {
				broken := path + ".broken"
				_ = os.Rename(path, broken)
				return q, fmt.Errorf("очередь повреждена, файл сохранён как %s: %w", broken, err)
			}
		}
	case os.IsNotExist(err):
	default:
		return nil, fmt.Errorf("чтение очереди %s: %w", path, err)
	}
	for _, it := range q.items {
		if it.ID > q.lastID {
			q.lastID = it.ID
		}
	}
	for i := range q.items {
		if q.items[i].ID == 0 {
			q.lastID++
			q.items[i].ID = q.lastID
		}
	}
	return q, nil
}

func (q *Queue) SetLastID(id int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if id > q.lastID {
		q.lastID = id
	}
}

func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *Queue) Add(s lastfm.Scrobble) error {
	_, err := q.Push(s)
	return err
}

func (q *Queue) Push(s lastfm.Scrobble) (Item, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.lastID++
	it := Item{ID: q.lastID, Scrobble: s, QueuedAt: time.Now().UTC()}
	q.items = append(q.items, it)
	if len(q.items) > q.max {
		q.items = q.items[len(q.items)-q.max:]
	}
	return it, q.saveLocked()
}

func (q *Queue) Head(n int) []Item {
	q.mu.Lock()
	defer q.mu.Unlock()
	if n > len(q.items) {
		n = len(q.items)
	}
	out := make([]Item, n)
	copy(out, q.items[:n])
	return out
}

func (q *Queue) Items() []Item {
	return q.Head(q.Len())
}

func (q *Queue) Drop(n int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if n <= 0 {
		return nil
	}
	if n > len(q.items) {
		n = len(q.items)
	}
	q.items = append([]Item(nil), q.items[n:]...)
	return q.saveLocked()
}

func (q *Queue) MarkAttempt(n int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if n > len(q.items) {
		n = len(q.items)
	}
	for i := 0; i < n; i++ {
		q.items[i].Attempts++
	}
	return q.saveLocked()
}

func (q *Queue) Remove(ids ...int64) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	drop := make(map[int64]bool, len(ids))
	for _, id := range ids {
		drop[id] = true
	}
	kept := make([]Item, 0, len(q.items))
	for _, it := range q.items {
		if !drop[it.ID] {
			kept = append(kept, it)
		}
	}
	n := len(q.items) - len(kept)
	if n == 0 {
		return 0, nil
	}
	q.items = kept
	return n, q.saveLocked()
}

func (q *Queue) MarkAttempts(ids ...int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	mark := make(map[int64]bool, len(ids))
	for _, id := range ids {
		mark[id] = true
	}
	for i := range q.items {
		if mark[q.items[i].ID] {
			q.items[i].Attempts++
		}
	}
	return q.saveLocked()
}

func (q *Queue) Clear() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = nil
	return q.saveLocked()
}

func (q *Queue) saveLocked() error {
	if q.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(q.items, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(q.path, data)
}
