package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	tagsFresh = 24 * time.Hour
	tagsRetry = 2 * time.Minute
	tagsLimit = 20 * time.Second
	tagsKeep  = 300
	tagsShown = 3
)

type tagCache struct {
	mu    sync.Mutex
	known map[string]*tagEntry
	busy  sync.WaitGroup
}

type tagEntry struct {
	tags    []string
	pending bool
	next    time.Time
}

func (s *service) tags(artist, title string) []string {
	if strings.TrimSpace(artist) == "" || strings.TrimSpace(title) == "" {
		return nil
	}
	key := lovedKey("", artist, title)
	c := &s.tagged
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.known[key]
	if e == nil {
		if c.known == nil {
			c.known = map[string]*tagEntry{}
		}
		if len(c.known) >= tagsKeep {
			for k, old := range c.known {
				if !old.pending {
					delete(c.known, k)
				}
			}
		}
		e = &tagEntry{}
		c.known[key] = e
	}
	if !e.pending && !time.Now().Before(e.next) {
		e.pending = true
		c.busy.Add(1)
		go s.fetchTags(key, artist, title)
	}
	return append([]string(nil), e.tags...)
}

func (s *service) fetchTags(key, artist, title string) {
	defer s.tagged.busy.Done()
	var tags []string
	err := errors.New("не заданы ключи приложения Last.fm")
	if client := s.newClient(s.config(), ""); client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), tagsLimit)
		tags, err = client.TopTags(ctx, artist, title, tagsShown)
		cancel()
	}
	s.tagged.mu.Lock()
	defer s.tagged.mu.Unlock()
	e := s.tagged.known[key]
	if e == nil {
		return
	}
	e.pending = false
	if err != nil {
		e.next = time.Now().Add(tagsRetry)
		s.log.Debug("не удалось получить теги Last.fm", "track", artist+" — "+title, "err", err)
		return
	}
	e.tags, e.next = tags, time.Now().Add(tagsFresh)
}
