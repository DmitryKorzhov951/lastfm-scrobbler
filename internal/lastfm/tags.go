package lastfm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

type tagItem struct {
	Name  string  `json:"name"`
	Count flexInt `json:"count"`
}

type tagList []tagItem

func (l *tagList) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '{' {
		var one tagItem
		if err := json.Unmarshal(b, &one); err != nil {
			return err
		}
		*l = tagList{one}
		return nil
	}
	var many []tagItem
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*l = many
	return nil
}

var junkTags = map[string]bool{"seen live": true}

func (c *Client) TopTags(ctx context.Context, artist, title string, limit int) ([]string, error) {
	artist, title = strings.TrimSpace(artist), strings.TrimSpace(title)
	if artist == "" {
		return nil, nil
	}
	if title != "" {
		tags, err := c.topTags(ctx, "track.getTopTags", url.Values{"artist": {artist}, "track": {title}}, artist, title, limit)
		if err != nil || len(tags) > 0 {
			return tags, err
		}
	}
	return c.topTags(ctx, "artist.getTopTags", url.Values{"artist": {artist}}, artist, title, limit)
}

func (c *Client) topTags(ctx context.Context, method string, params url.Values, artist, title string, limit int) ([]string, error) {
	params.Set("autocorrect", "1")
	body, err := c.call(ctx, method, params, requestOptions{})
	if err != nil {
		if notFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var out struct {
		TopTags struct {
			Tag tagList `json:"tag"`
		} `json:"toptags"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("%s: разбор ответа: %w", method, err)
	}
	skip := map[string]bool{squash(artist): true, squash(title): true}
	var tags []string
	for _, t := range out.TopTags.Tag {
		name := strings.ToLower(strings.Join(strings.Fields(t.Name), " "))
		if name == "" || junkTags[name] || skip[squash(name)] {
			continue
		}
		skip[squash(name)] = true
		tags = append(tags, name)
		if len(tags) == limit {
			break
		}
	}
	return tags, nil
}

func squash(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
