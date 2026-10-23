package lastfm

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultEndpoint = "https://ws.audioscrobbler.com/2.0/"
	MaxBatch        = 50
	maxBodySize     = 1 << 20
)

type Client struct {
	APIKey     string
	APISecret  string
	SessionKey string
	Endpoint   string
	UserAgent  string
	HTTP       *http.Client
}

func New(apiKey, apiSecret string) *Client {
	return &Client{
		APIKey:    apiKey,
		APISecret: apiSecret,
		Endpoint:  DefaultEndpoint,
		UserAgent: "lastfm-scrobbler",
		HTTP:      &http.Client{Timeout: 20 * time.Second},
	}
}

type Error struct {
	Code       int
	Message    string
	HTTPStatus int
}

func (e *Error) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("last.fm error %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("last.fm http %d: %s", e.HTTPStatus, e.Message)
}

const (
	ErrCodeInvalidService  = 2
	ErrCodeAuthFailed      = 4
	ErrCodeInvalidParams   = 6
	ErrCodeOperationFailed = 8
	ErrCodeInvalidSession  = 9
	ErrCodeInvalidAPIKey   = 10
	ErrCodeServiceOffline  = 11
	ErrCodeInvalidSig      = 13
	ErrCodeTokenNotAuth    = 14
	ErrCodeTokenExpired    = 15
	ErrCodeServiceUnavail  = 16
	ErrCodeRateLimit       = 29
)

func (e *Error) Temporary() bool {
	switch e.Code {
	case ErrCodeOperationFailed, ErrCodeServiceOffline, ErrCodeServiceUnavail, ErrCodeRateLimit:
		return true
	}
	return e.HTTPStatus >= 500 || e.HTTPStatus == http.StatusTooManyRequests
}

func (e *Error) NeedsAuth() bool {
	switch e.Code {
	case ErrCodeAuthFailed, ErrCodeInvalidSession, ErrCodeInvalidAPIKey, ErrCodeInvalidSig:
		return true
	}
	return false
}

func Retryable(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *Error
	if asError(err, &apiErr) {
		return apiErr.Temporary() || apiErr.NeedsAuth()
	}
	return true
}

func Sign(secret string, params url.Values) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		switch k {
		case "format", "callback", "api_sig":
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params.Get(k))
	}
	b.WriteString(secret)

	sum := md5.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

type requestOptions struct {
	post   bool
	signed bool
}

func (c *Client) endpoint() string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	return DefaultEndpoint
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) call(ctx context.Context, method string, params url.Values, opt requestOptions) ([]byte, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("не задан api_key")
	}
	if params == nil {
		params = url.Values{}
	}
	params.Set("method", method)
	params.Set("api_key", c.APIKey)

	if opt.signed {
		if c.APISecret == "" {
			return nil, fmt.Errorf("не задан api_secret")
		}
		params.Set("api_sig", Sign(c.APISecret, params))
	}
	params.Set("format", "json")

	var req *http.Request
	var err error
	if opt.post {
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), strings.NewReader(params.Encode()))
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	} else {
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint()+"?"+params.Encode(), nil)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("%s: чтение ответа: %w", method, err)
	}

	var apiErr struct {
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Error != 0 {
		return nil, &Error{Code: apiErr.Error, Message: apiErr.Message, HTTPStatus: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &Error{HTTPStatus: resp.StatusCode, Message: strings.TrimSpace(string(body))}
	}
	return body, nil
}

type Session struct {
	Name       string `json:"name"`
	Key        string `json:"key"`
	Subscriber int    `json:"subscriber"`
}

func (c *Client) GetToken(ctx context.Context) (string, error) {
	body, err := c.call(ctx, "auth.getToken", url.Values{}, requestOptions{signed: true})
	if err != nil {
		return "", err
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("auth.getToken: разбор ответа: %w", err)
	}
	if out.Token == "" {
		return "", fmt.Errorf("auth.getToken: пустой токен")
	}
	return out.Token, nil
}

func (c *Client) AuthURL(token string) string {
	v := url.Values{}
	v.Set("api_key", c.APIKey)
	v.Set("token", token)
	return "https://www.last.fm/api/auth/?" + v.Encode()
}

func (c *Client) GetSession(ctx context.Context, token string) (Session, error) {
	params := url.Values{}
	params.Set("token", token)
	body, err := c.call(ctx, "auth.getSession", params, requestOptions{signed: true})
	if err != nil {
		return Session{}, err
	}
	var out struct {
		Session struct {
			Name       string  `json:"name"`
			Key        string  `json:"key"`
			Subscriber flexInt `json:"subscriber"`
		} `json:"session"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return Session{}, fmt.Errorf("auth.getSession: разбор ответа: %w", err)
	}
	if out.Session.Key == "" {
		return Session{}, fmt.Errorf("auth.getSession: пустой ключ сессии")
	}
	return Session{Name: out.Session.Name, Key: out.Session.Key, Subscriber: int(out.Session.Subscriber)}, nil
}

func trackParams(params url.Values, t Track, suffix string) {
	params.Set("artist"+suffix, t.Artist)
	params.Set("track"+suffix, t.Title)
	if t.Album != "" {
		params.Set("album"+suffix, t.Album)
	}
	if t.AlbumArtist != "" && !strings.EqualFold(t.AlbumArtist, t.Artist) {
		params.Set("albumArtist"+suffix, t.AlbumArtist)
	}
	if secs := int(t.Duration.Seconds()); secs > 0 {
		params.Set("duration"+suffix, strconv.Itoa(secs))
	}
	if t.TrackNumber > 0 {
		params.Set("trackNumber"+suffix, strconv.Itoa(t.TrackNumber))
	}
	if t.MBID != "" {
		params.Set("mbid"+suffix, t.MBID)
	}
}

func (c *Client) UpdateNowPlaying(ctx context.Context, t Track) error {
	if c.SessionKey == "" {
		return fmt.Errorf("нет ключа сессии: выполните «lastfm-scrobbler auth»")
	}
	if !t.Valid() {
		return fmt.Errorf("недостаточно метаданных для updateNowPlaying")
	}
	params := url.Values{}
	trackParams(params, t, "")
	params.Set("sk", c.SessionKey)
	_, err := c.call(ctx, "track.updateNowPlaying", params, requestOptions{post: true, signed: true})
	return err
}

type ScrobbleResult struct {
	Accepted       int
	Ignored        int
	IgnoredReasons []string
	IgnoredIndex   []int
}

func (c *Client) Scrobble(ctx context.Context, items []Scrobble) (ScrobbleResult, error) {
	if c.SessionKey == "" {
		return ScrobbleResult{}, fmt.Errorf("нет ключа сессии: выполните «lastfm-scrobbler auth»")
	}
	if len(items) == 0 {
		return ScrobbleResult{}, nil
	}
	if len(items) > MaxBatch {
		return ScrobbleResult{}, fmt.Errorf("за раз можно отправить не более %d скробблов", MaxBatch)
	}

	params := url.Values{}
	for i, it := range items {
		suffix := "[" + strconv.Itoa(i) + "]"
		trackParams(params, it.Track, suffix)
		params.Set("timestamp"+suffix, strconv.FormatInt(it.Timestamp.Unix(), 10))
		if it.Radio {
			params.Set("chosenByUser"+suffix, "0")
		}
	}
	params.Set("sk", c.SessionKey)

	body, err := c.call(ctx, "track.scrobble", params, requestOptions{post: true, signed: true})
	if err != nil {
		return ScrobbleResult{}, err
	}
	return parseScrobbleResult(body)
}

func parseScrobbleResult(body []byte) (ScrobbleResult, error) {
	var out struct {
		Scrobbles struct {
			Attr struct {
				Accepted flexInt `json:"accepted"`
				Ignored  flexInt `json:"ignored"`
			} `json:"@attr"`
			Scrobble json.RawMessage `json:"scrobble"`
		} `json:"scrobbles"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return ScrobbleResult{}, fmt.Errorf("track.scrobble: разбор ответа: %w", err)
	}

	res := ScrobbleResult{
		Accepted: int(out.Scrobbles.Attr.Accepted),
		Ignored:  int(out.Scrobbles.Attr.Ignored),
	}
	if res.Ignored == 0 || len(out.Scrobbles.Scrobble) == 0 {
		return res, nil
	}

	type scrobbleEntry struct {
		IgnoredMessage struct {
			Code flexInt `json:"code"`
			Text string  `json:"#text"`
		} `json:"ignoredMessage"`
		Track struct {
			Text string `json:"#text"`
		} `json:"track"`
		Artist struct {
			Text string `json:"#text"`
		} `json:"artist"`
	}
	var entries []scrobbleEntry
	if err := json.Unmarshal(out.Scrobbles.Scrobble, &entries); err != nil {
		var single scrobbleEntry
		if err := json.Unmarshal(out.Scrobbles.Scrobble, &single); err != nil {
			return res, nil
		}
		entries = []scrobbleEntry{single}
	}
	for i, e := range entries {
		if e.IgnoredMessage.Code == 0 {
			continue
		}
		res.IgnoredIndex = append(res.IgnoredIndex, i)
		reason := strings.TrimSpace(e.IgnoredMessage.Text)
		if reason == "" {
			reason = "код " + strconv.Itoa(int(e.IgnoredMessage.Code))
		}
		res.IgnoredReasons = append(res.IgnoredReasons,
			fmt.Sprintf("%s — %s: %s", e.Artist.Text, e.Track.Text, reason))
	}
	return res, nil
}

type flexInt int64

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*f = flexInt(n)
	return nil
}
