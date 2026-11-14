package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	lovedFresh = 2 * time.Minute
	lovedRetry = time.Minute
	lovedLimit = 20 * time.Second
	lovedKeep  = 200
)

type lovedCache struct {
	mu    sync.Mutex
	known map[string]*lovedEntry
	busy  sync.WaitGroup
}

type lovedEntry struct {
	loved   bool
	checked bool
	pending bool
	next    time.Time
}

func lovedKey(user, artist, title string) string {
	n := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
	return n(user) + "\x00" + n(artist) + "\x00" + n(title)
}

func (s *service) loved(artist, title string, playing bool) bool {
	acc := s.account()
	if acc.Key == "" || acc.Username == "" || strings.TrimSpace(artist) == "" || strings.TrimSpace(title) == "" {
		return false
	}
	key := lovedKey(acc.Username, artist, title)
	c := &s.love
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.known[key]
	if e == nil {
		if c.known == nil {
			c.known = map[string]*lovedEntry{}
		}
		if len(c.known) >= lovedKeep {
			for k, old := range c.known {
				if !old.pending {
					delete(c.known, k)
				}
			}
		}
		e = &lovedEntry{}
		c.known[key] = e
	}
	if !e.pending && (playing || !e.checked) && !time.Now().Before(e.next) {
		e.pending = true
		c.busy.Add(1)
		go s.checkLoved(key, acc.Username, artist, title)
	}
	return e.loved
}

func (s *service) checkLoved(key, user, artist, title string) {
	defer s.love.busy.Done()
	loved, err := false, errors.New("не заданы ключи приложения Last.fm")
	if client := s.newClient(s.config(), ""); client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), lovedLimit)
		loved, err = client.Loved(ctx, user, artist, title)
		cancel()
	}
	s.love.mu.Lock()
	defer s.love.mu.Unlock()
	e := s.love.known[key]
	if e == nil {
		return
	}
	e.pending = false
	if err != nil {
		e.next = time.Now().Add(lovedRetry)
		s.log.Debug("не удалось узнать, любимый ли трек", "track", artist+" — "+title, "err", err)
		return
	}
	e.loved, e.checked = loved, true
	e.next = time.Now().Add(lovedFresh)
}
