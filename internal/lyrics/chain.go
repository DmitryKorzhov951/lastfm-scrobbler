package lyrics

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

type Fixer func(ctx context.Context, artist, title string) (string, string, error)

type Chain struct {
	Sources []Finder
	Fix     Fixer
}

type Incomplete struct {
	Err error
}

func (e *Incomplete) Error() string {
	return "текст песни не найден, часть сайтов не ответила: " + e.Err.Error()
}

func (e *Incomplete) Unwrap() error { return e.Err }

func (c *Chain) Find(ctx context.Context, q Query) (Lyrics, error) {
	queries := []Query{q}
	if main := mainArtist(q.Artist); main != "" && squash(main) != squash(q.Artist) {
		alt := q
		alt.Artist = main
		queries = append(queries, alt)
	}
	fixed := c.Fix == nil
	var failed error
	answered := false
	for _, find := range c.Sources {
		for i := 0; i < len(queries) || !fixed; i++ {
			if i == len(queries) {
				fixed = true
				fq, ok := c.fixed(ctx, q, queries)
				if !ok {
					break
				}
				queries = append(queries, fq)
			}
			l, err := find(ctx, queries[i])
			if err == nil && !l.Empty() {
				return l, nil
			}
			if err != nil && !errors.Is(err, ErrNotFound) {
				if failed == nil {
					failed = err
				}
				break
			}
			answered = true
		}
		if ctx.Err() != nil {
			break
		}
	}
	switch {
	case failed == nil:
		return Lyrics{}, ErrNotFound
	case answered:
		return Lyrics{}, &Incomplete{Err: failed}
	}
	return Lyrics{}, failed
}

func (c *Chain) fixed(ctx context.Context, q Query, have []Query) (Query, bool) {
	artist, title, err := c.Fix(ctx, q.Artist, q.Title)
	artist, title = strings.TrimSpace(artist), strings.TrimSpace(title)
	if err != nil || artist == "" || title == "" {
		return Query{}, false
	}
	for _, h := range have {
		if squash(h.Artist) == squash(artist) && squash(CleanTitle(h.Title)) == squash(CleanTitle(title)) {
			return Query{}, false
		}
	}
	q.Artist, q.Title = artist, title
	return q, true
}

var artistSepRe = regexp.MustCompile(`(?i)\s*(?:[,;&+]|\s(?:feat\.?|ft\.?|featuring|with|x|vs\.?|and|и)\s)\s*`)

func mainArtist(s string) string {
	s = strings.TrimSpace(s)
	if loc := artistSepRe.FindStringIndex(s); loc != nil && loc[0] > 0 {
		return strings.TrimSpace(s[:loc[0]])
	}
	return s
}
