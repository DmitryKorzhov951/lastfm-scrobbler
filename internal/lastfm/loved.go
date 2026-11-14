package lastfm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func (c *Client) Loved(ctx context.Context, user, artist, title string) (bool, error) {
	user, artist, title = strings.TrimSpace(user), strings.TrimSpace(artist), strings.TrimSpace(title)
	if user == "" || artist == "" || title == "" {
		return false, nil
	}
	params := url.Values{}
	params.Set("artist", artist)
	params.Set("track", title)
	params.Set("username", user)
	params.Set("autocorrect", "1")
	body, err := c.call(ctx, "track.getInfo", params, requestOptions{})
	if err != nil {
		if notFound(err) {
			return false, nil
		}
		return false, err
	}
	var out struct {
		Track struct {
			UserLoved flexInt `json:"userloved"`
		} `json:"track"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return false, fmt.Errorf("track.getInfo: разбор ответа: %w", err)
	}
	return out.Track.UserLoved == 1, nil
}
