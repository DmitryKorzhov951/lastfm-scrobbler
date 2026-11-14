package lastfm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type SimilarArtist struct {
	Name  string
	Match float64
}

type SimilarTrack struct {
	Title  string
	Artist string
	Match  float64
}

type oneOrMany[T any] []T

func (l *oneOrMany[T]) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '{' {
		var one T
		if err := json.Unmarshal(b, &one); err != nil {
			return err
		}
		*l = oneOrMany[T]{one}
		return nil
	}
	var many []T
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*l = many
	return nil
}

type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = flexFloat(v)
	return nil
}

func (c *Client) SimilarArtists(ctx context.Context, artist string, limit int) ([]SimilarArtist, error) {
	artist = strings.TrimSpace(artist)
	if artist == "" {
		return nil, nil
	}
	params := url.Values{"artist": {artist}, "autocorrect": {"1"}, "limit": {strconv.Itoa(limit)}}
	var out struct {
		Similar struct {
			Artist oneOrMany[struct {
				Name  string    `json:"name"`
				Match flexFloat `json:"match"`
			}] `json:"artist"`
		} `json:"similarartists"`
	}
	if found, err := c.getJSON(ctx, "artist.getSimilar", params, &out); err != nil || !found {
		return nil, err
	}
	list := make([]SimilarArtist, 0, len(out.Similar.Artist))
	for _, a := range out.Similar.Artist {
		if name := strings.TrimSpace(a.Name); name != "" {
			list = append(list, SimilarArtist{Name: name, Match: float64(a.Match)})
		}
	}
	return list, nil
}

func (c *Client) SimilarTracks(ctx context.Context, artist, title string, limit int) ([]SimilarTrack, error) {
	artist, title = strings.TrimSpace(artist), strings.TrimSpace(title)
	if artist == "" || title == "" {
		return nil, nil
	}
	params := url.Values{"artist": {artist}, "track": {title}, "autocorrect": {"1"}, "limit": {strconv.Itoa(limit)}}
	var out struct {
		Similar struct {
			Track oneOrMany[struct {
				Name   string    `json:"name"`
				Match  flexFloat `json:"match"`
				Artist struct {
					Name string `json:"name"`
				} `json:"artist"`
			}] `json:"track"`
		} `json:"similartracks"`
	}
	if found, err := c.getJSON(ctx, "track.getSimilar", params, &out); err != nil || !found {
		return nil, err
	}
	list := make([]SimilarTrack, 0, len(out.Similar.Track))
	for _, t := range out.Similar.Track {
		name, who := strings.TrimSpace(t.Name), strings.TrimSpace(t.Artist.Name)
		if name != "" && who != "" {
			list = append(list, SimilarTrack{Title: name, Artist: who, Match: float64(t.Match)})
		}
	}
	return list, nil
}

func (c *Client) getJSON(ctx context.Context, method string, params url.Values, out any) (bool, error) {
	body, err := c.call(ctx, method, params, requestOptions{})
	if err != nil {
		if notFound(err) {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return false, fmt.Errorf("%s: разбор ответа: %w", method, err)
	}
	return true, nil
}
