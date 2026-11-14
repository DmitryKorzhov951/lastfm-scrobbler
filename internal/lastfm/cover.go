package lastfm

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
)

const placeholderImage = "2a96cbd8b46e442fc41c2b86b821562f"

type imageField struct {
	Text string `json:"#text"`
	Size string `json:"size"`
}

var imageRank = map[string]int{"small": 1, "medium": 2, "large": 3, "extralarge": 4, "mega": 5}

func pickImage(list []imageField) string {
	best, rank := "", -1
	for _, im := range list {
		u := strings.TrimSpace(im.Text)
		if u == "" || strings.Contains(u, placeholderImage) {
			continue
		}
		if r := imageRank[im.Size]; r > rank {
			best, rank = u, r
		}
	}
	return best
}

func (c *Client) CoverURL(ctx context.Context, artist, title, album string) (string, error) {
	artist, title, album = strings.TrimSpace(artist), strings.TrimSpace(title), strings.TrimSpace(album)
	if artist == "" {
		return "", nil
	}
	if album != "" {
		u, err := c.albumImage(ctx, artist, album)
		if err != nil && !notFound(err) {
			return "", err
		}
		if u != "" {
			return u, nil
		}
	}
	if title == "" {
		return "", nil
	}
	u, err := c.trackImage(ctx, artist, title)
	if err != nil && !notFound(err) {
		return "", err
	}
	return u, nil
}

func notFound(err error) bool {
	e, ok := AsError(err)
	return ok && e.Code == ErrCodeInvalidParams
}

func (c *Client) albumImage(ctx context.Context, artist, album string) (string, error) {
	params := url.Values{}
	params.Set("artist", artist)
	params.Set("album", album)
	params.Set("autocorrect", "1")
	body, err := c.call(ctx, "album.getInfo", params, requestOptions{})
	if err != nil {
		return "", err
	}
	var out struct {
		Album struct {
			Image []imageField `json:"image"`
		} `json:"album"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return pickImage(out.Album.Image), nil
}

func (c *Client) trackImage(ctx context.Context, artist, title string) (string, error) {
	params := url.Values{}
	params.Set("artist", artist)
	params.Set("track", title)
	params.Set("autocorrect", "1")
	body, err := c.call(ctx, "track.getInfo", params, requestOptions{})
	if err != nil {
		return "", err
	}
	var out struct {
		Track struct {
			Album struct {
				Image []imageField `json:"image"`
			} `json:"album"`
		} `json:"track"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return pickImage(out.Track.Album.Image), nil
}
