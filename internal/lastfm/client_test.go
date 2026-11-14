package lastfm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSign(t *testing.T) {
	params := url.Values{}
	params.Set("method", "auth.getToken")
	params.Set("api_key", "abc123")
	params.Set("format", "json")

	got := Sign("s3cret", params)
	want := "a26a54959b95f1ea73e2bc0f72a38ced"
	if got != want {
		t.Fatalf("Sign() = %s, ожидалось %s", got, want)
	}
}

func TestSignIgnoresFormatAndCallback(t *testing.T) {
	base := url.Values{}
	base.Set("method", "track.scrobble")
	base.Set("api_key", "k")

	withExtra := url.Values{}
	for k, v := range base {
		withExtra[k] = v
	}
	withExtra.Set("format", "json")
	withExtra.Set("callback", "cb")
	withExtra.Set("api_sig", "устаревшая подпись")

	if Sign("secret", base) != Sign("secret", withExtra) {
		t.Fatal("format, callback и api_sig не должны влиять на подпись")
	}
}

func TestUpdateNowPlayingRequest(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("метод %s, ожидался POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		got, _ = url.ParseQuery(string(body))
		io.WriteString(w, `{"nowplaying":{"track":{"#text":"Creep"}}}`)
	}))
	defer srv.Close()

	c := New("KEY", "SECRET")
	c.Endpoint = srv.URL
	c.SessionKey = "SESSION"

	err := c.UpdateNowPlaying(context.Background(), Track{
		Artist:   "Radiohead",
		Title:    "Creep",
		Duration: 238 * time.Second,
	})
	if err != nil {
		t.Fatalf("UpdateNowPlaying: %v", err)
	}

	if got.Get("method") != "track.updateNowPlaying" {
		t.Errorf("method = %q", got.Get("method"))
	}
	if got.Get("artist") != "Radiohead" || got.Get("track") != "Creep" {
		t.Errorf("метаданные не переданы: %v", got)
	}
	if got.Get("duration") != "238" {
		t.Errorf("duration = %q, ожидалось 238", got.Get("duration"))
	}
	if got.Get("sk") != "SESSION" {
		t.Errorf("не передан ключ сессии")
	}
	if got.Get("format") != "json" {
		t.Errorf("format = %q", got.Get("format"))
	}

	sent := url.Values{}
	for k, v := range got {
		sent[k] = v
	}
	sig := sent.Get("api_sig")
	if sig == "" {
		t.Fatal("подпись не передана")
	}
	if want := Sign("SECRET", sent); sig != want {
		t.Errorf("api_sig = %s, ожидалось %s", sig, want)
	}
}

func TestScrobbleBatchParams(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got, _ = url.ParseQuery(string(body))
		io.WriteString(w, `{"scrobbles":{"@attr":{"accepted":2,"ignored":0},"scrobble":[]}}`)
	}))
	defer srv.Close()

	c := New("KEY", "SECRET")
	c.Endpoint = srv.URL
	c.SessionKey = "SESSION"

	ts := time.Unix(1700000000, 0)
	res, err := c.Scrobble(context.Background(), []Scrobble{
		{Track: Track{Artist: "A", Title: "One", Album: "Alb", Duration: time.Minute}, Timestamp: ts},
		{Track: Track{Artist: "B", Title: "Two"}, Timestamp: ts.Add(time.Minute)},
	})
	if err != nil {
		t.Fatalf("Scrobble: %v", err)
	}
	if res.Accepted != 2 || res.Ignored != 0 {
		t.Errorf("результат = %+v", res)
	}

	if got.Get("artist[0]") != "A" || got.Get("track[0]") != "One" || got.Get("album[0]") != "Alb" {
		t.Errorf("первый трек передан неверно: %v", got)
	}
	if got.Get("timestamp[0]") != "1700000000" {
		t.Errorf("timestamp[0] = %q", got.Get("timestamp[0]"))
	}
	if got.Get("artist[1]") != "B" || got.Get("track[1]") != "Two" {
		t.Errorf("второй трек передан неверно: %v", got)
	}
	if got.Has("album[1]") || got.Has("duration[1]") {
		t.Errorf("переданы пустые поля: %v", got)
	}
}

func TestScrobbleTooLarge(t *testing.T) {
	c := New("KEY", "SECRET")
	c.SessionKey = "SESSION"
	items := make([]Scrobble, MaxBatch+1)
	if _, err := c.Scrobble(context.Background(), items); err == nil {
		t.Fatal("ожидалась ошибка при превышении размера пакета")
	}
}

func TestAPIErrorParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"error":9,"message":"Invalid session key - Please re-authenticate"}`)
	}))
	defer srv.Close()

	c := New("KEY", "SECRET")
	c.Endpoint = srv.URL
	c.SessionKey = "SESSION"

	_, err := c.Scrobble(context.Background(), []Scrobble{
		{Track: Track{Artist: "A", Title: "B"}, Timestamp: time.Now()},
	})
	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("ожидалась *lastfm.Error, получено %T: %v", err, err)
	}
	if apiErr.Code != ErrCodeInvalidSession {
		t.Errorf("код = %d", apiErr.Code)
	}
	if !apiErr.NeedsAuth() {
		t.Error("ошибка 9 должна требовать повторной авторизации")
	}
	if !Retryable(err) {
		t.Error("скроббл должен остаться в очереди до повторной авторизации")
	}
}

func TestRetryable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"нет ошибки", nil, false},
		{"сервис недоступен", &Error{Code: ErrCodeServiceUnavail}, true},
		{"превышен лимит", &Error{Code: ErrCodeRateLimit}, true},
		{"ошибка сервера", &Error{HTTPStatus: 502}, true},
		{"неверные параметры", &Error{Code: ErrCodeInvalidParams}, false},
		{"сетевой сбой", io.ErrUnexpectedEOF, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Retryable(tc.err); got != tc.want {
				t.Errorf("Retryable(%v) = %v, ожидалось %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestParseScrobbleResultSingleAndArray(t *testing.T) {
	single := []byte(`{"scrobbles":{"@attr":{"accepted":"0","ignored":"1"},
		"scrobble":{"artist":{"#text":"A"},"track":{"#text":"B"},
		"ignoredMessage":{"code":"1","#text":"Artist was ignored"}}}}`)
	res, err := parseScrobbleResult(single)
	if err != nil {
		t.Fatalf("одиночный скроббл: %v", err)
	}
	if res.Accepted != 0 || res.Ignored != 1 {
		t.Errorf("результат = %+v", res)
	}
	if len(res.IgnoredReasons) != 1 || !strings.Contains(res.IgnoredReasons[0], "Artist was ignored") {
		t.Errorf("причина отклонения не разобрана: %+v", res.IgnoredReasons)
	}

	array := []byte(`{"scrobbles":{"@attr":{"accepted":2,"ignored":0},
		"scrobble":[{"track":{"#text":"A"}},{"track":{"#text":"B"}}]}}`)
	res, err = parseScrobbleResult(array)
	if err != nil {
		t.Fatalf("массив скробблов: %v", err)
	}
	if res.Accepted != 2 || res.Ignored != 0 {
		t.Errorf("результат = %+v", res)
	}

	mixed := []byte(`{"scrobbles":{"@attr":{"accepted":2,"ignored":1},
		"scrobble":[{"track":{"#text":"A"},"ignoredMessage":{"code":"0","#text":""}},
		            {"track":{"#text":"B"},"ignoredMessage":{"code":"3","#text":"Timestamp too old"}},
		            {"track":{"#text":"C"},"ignoredMessage":{"code":"0","#text":""}}]}}`)
	res, err = parseScrobbleResult(mixed)
	if err != nil {
		t.Fatalf("смешанный пакет: %v", err)
	}
	if len(res.IgnoredIndex) != 1 || res.IgnoredIndex[0] != 1 {
		t.Errorf("отклонён второй скроббл пакета, получено %v", res.IgnoredIndex)
	}
}

func TestGetSessionAndToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("method") {
		case "auth.getToken":
			io.WriteString(w, `{"token":"TOKEN123"}`)
		case "auth.getSession":
			io.WriteString(w, `{"session":{"name":"user","key":"SK","subscriber":0}}`)
		default:
			t.Errorf("неожиданный метод %q", r.URL.Query().Get("method"))
		}
	}))
	defer srv.Close()

	c := New("KEY", "SECRET")
	c.Endpoint = srv.URL

	token, err := c.GetToken(context.Background())
	if err != nil || token != "TOKEN123" {
		t.Fatalf("GetToken = %q, %v", token, err)
	}
	if u := c.AuthURL(token); !strings.Contains(u, "token=TOKEN123") || !strings.Contains(u, "api_key=KEY") {
		t.Errorf("AuthURL = %s", u)
	}

	s, err := c.GetSession(context.Background(), token)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s.Name != "user" || s.Key != "SK" {
		t.Errorf("сессия = %+v", s)
	}
}

func TestScrobbleWithoutSession(t *testing.T) {
	c := New("KEY", "SECRET")
	if _, err := c.Scrobble(context.Background(), []Scrobble{{Track: Track{Artist: "A", Title: "B"}}}); err == nil {
		t.Fatal("без ключа сессии отправка должна завершаться ошибкой")
	}
	if err := c.UpdateNowPlaying(context.Background(), Track{Artist: "A", Title: "B"}); err == nil {
		t.Fatal("без ключа сессии «сейчас играет» должно завершаться ошибкой")
	}
}

func TestСкробблСРадиоОтмечается(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		got = r.PostForm
		w.Write([]byte(`{"scrobbles":{"@attr":{"accepted":2,"ignored":0}}}`))
	}))
	defer srv.Close()
	c := New("key", "secret")
	c.Endpoint = srv.URL
	c.SessionKey = "sk"
	ts := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	_, err := c.Scrobble(context.Background(), []Scrobble{
		{Track: Track{Artist: "Кино", Title: "Кукушка"}, Timestamp: ts},
		{Track: Track{Artist: "MGK", Title: "bloody valentine"}, Timestamp: ts, Radio: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Has("chosenByUser[0]") || got.Get("chosenByUser[1]") != "0" {
		t.Errorf("chosenByUser: %v", got)
	}
}

func TestRecentTracksЗапросИРазбор(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("метод %s, ожидался GET", r.Method)
		}
		got = r.URL.Query()
		io.WriteString(w, `{"recenttracks":{"track":[
			{"artist":{"mbid":"","#text":"Кино"},"album":{"#text":"Группа крови"},"name":"Группа крови",
			 "@attr":{"nowplaying":"true"}},
			{"artist":{"#text":"Земфира"},"album":{"#text":""},"name":"Хочешь?","date":{"uts":"1793890305","#text":"05 Nov 2026, 14:51"}}
		],"@attr":{"user":"korzhov","totalPages":"7","page":"2","perPage":"200","total":"1301"}}}`)
	}))
	defer srv.Close()

	c := New("KEY", "SECRET")
	c.Endpoint = srv.URL
	from, to := time.Unix(1700000000, 0), time.Unix(1793890400, 0)
	p, err := c.RecentTracks(context.Background(), "korzhov", 2, 0, from, to)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"method": "user.getRecentTracks", "user": "korzhov", "page": "2",
		"limit": "200", "from": "1700000000", "to": "1793890400", "api_key": "KEY"} {
		if got.Get(k) != v {
			t.Errorf("%s = %q, ожидалось %q", k, got.Get(k), v)
		}
	}
	if got.Get("api_sig") != "" || got.Get("sk") != "" {
		t.Error("запрос истории не нужно подписывать")
	}
	if p.Page != 2 || p.TotalPages != 7 || p.Total != 1301 || len(p.Tracks) != 2 {
		t.Fatalf("страница: %+v", p)
	}
	if !p.Tracks[0].NowPlaying || !p.Tracks[0].At.IsZero() || p.Tracks[0].Artist != "Кино" {
		t.Errorf("играющий сейчас трек: %+v", p.Tracks[0])
	}
	if p.Tracks[1].Title != "Хочешь?" || p.Tracks[1].At.Unix() != 1793890305 || p.Tracks[1].Album != "" {
		t.Errorf("прослушанный трек: %+v", p.Tracks[1])
	}
}

func TestRecentTracksОдинТрекИПустаяИстория(t *testing.T) {
	for body, want := range map[string]int{
		`{"recenttracks":{"track":{"artist":{"#text":"Кино"},"name":"Кукушка","date":{"uts":"1"}},"@attr":{"totalPages":"1","total":"1"}}}`: 1,
		`{"recenttracks":{"track":[],"@attr":{"totalPages":"0","total":"0"}}}`:                                                              0,
	} {
		p, err := parseRecentTracks([]byte(body))
		if err != nil || len(p.Tracks) != want {
			t.Errorf("%s: %d треков, %v", body, len(p.Tracks), err)
		}
	}
}
