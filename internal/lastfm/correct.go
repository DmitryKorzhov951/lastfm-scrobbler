package lastfm

import (
	"context"
	"net/url"
	"strings"
)

func (c *Client) Correct(ctx context.Context, artist, title string) (string, string, error) {
	artist, title = strings.TrimSpace(artist), strings.TrimSpace(title)
	if artist == "" || title == "" {
		return "", "", nil
	}
	var out struct {
		Track struct {
			Name   string `json:"name"`
			Artist struct {
				Name string `json:"name"`
			} `json:"artist"`
		} `json:"track"`
	}
	params := url.Values{"artist": {artist}, "track": {title}, "autocorrect": {"1"}}
	if found, err := c.getJSON(ctx, "track.getInfo", params, &out); err != nil || !found {
		return "", "", err
	}
	return strings.TrimSpace(out.Track.Artist.Name), strings.TrimSpace(out.Track.Name), nil
}
