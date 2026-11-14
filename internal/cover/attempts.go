package cover

import (
	"sync"
	"time"
)

const (
	attemptsMax   = 3
	attemptsRetry = 2 * time.Second
)

type Attempts struct {
	mu   sync.Mutex
	m    map[string]*attempt
	seen map[string]bool
}

type attempt struct {
	data  []byte
	tries int
	next  time.Time
}

func NewAttempts() *Attempts {
	return &Attempts{m: map[string]*attempt{}, seen: map[string]bool{}}
}

func (a *Attempts) Due(key string, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.seen[key] = true
	t := a.m[key]
	if t == nil {
		a.m[key] = &attempt{}
		return true
	}
	return t.tries < attemptsMax && !now.Before(t.next)
}

func (a *Attempts) Put(key string, data []byte, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t := a.m[key]
	if t == nil {
		return
	}
	t.tries++
	t.next = now.Add(attemptsRetry * time.Duration(t.tries))
	if len(data) > 0 && len(data) <= MaxSize && Sniff(data) != "" {
		t.data = data
	}
}

func (a *Attempts) Get(key string) []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	if t := a.m[key]; t != nil {
		return t.data
	}
	return nil
}

func (a *Attempts) Sweep() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for k := range a.m {
		if !a.seen[k] {
			delete(a.m, k)
		}
	}
	clear(a.seen)
}
