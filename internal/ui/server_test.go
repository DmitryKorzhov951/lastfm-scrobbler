package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testServer(t *testing.T, b Backend) (*Server, http.Handler) {
	t.Helper()
	s, err := Listen(b, "127.0.0.1:0", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.ln.Close() })
	return s, s.Handler()
}

func do(t *testing.T, h http.Handler, s *Server, method, path, body string, token bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "127.0.0.1:" + s.port
	if token {
		req.Header.Set("X-Token", s.token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestТолькоПетлевойАдрес(t *testing.T) {
	if _, err := Listen(NewDemo(true, false), "0.0.0.0:0", nil); err == nil {
		t.Error("сервер окна не должен слушать все адреса")
	}
}

func TestАдресОкнаСодержитТокенПослеРешётки(t *testing.T) {
	s, _ := testServer(t, NewDemo(true, false))
	if !strings.HasPrefix(s.URL(), "http://127.0.0.1:"+s.port+"/#t=") || !strings.HasSuffix(s.URL(), s.token) {
		t.Errorf("адрес окна: %s", s.URL())
	}
}

func TestЧужойHostОтвергается(t *testing.T) {
	s, h := testServer(t, NewDemo(true, false))
	req := httptest.NewRequest("GET", "/api/state", nil)
	req.Host = "evil.example:" + s.port
	req.Header.Set("X-Token", s.token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMisdirectedRequest {
		t.Errorf("запрос с чужим Host: код %d", rec.Code)
	}
}

func TestБезТокенаНетДоступа(t *testing.T) {
	s, h := testServer(t, NewDemo(true, false))
	if rec := do(t, h, s, "GET", "/api/state", "", false); rec.Code != http.StatusUnauthorized {
		t.Errorf("без токена: код %d", rec.Code)
	}
	req := httptest.NewRequest("POST", "/api/queue/clear", nil)
	req.Host = "127.0.0.1:" + s.port
	req.Header.Set("X-Token", "чужой")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("с неверным токеном: код %d", rec.Code)
	}
	if rec := do(t, h, s, "GET", "/", "", false); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Следилка за музыкой") {
		t.Errorf("страница окна: код %d", rec.Code)
	} else if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("нет политики безопасности содержимого: %q", csp)
	}
}

func TestСостояниеИОчередь(t *testing.T) {
	demo := NewDemo(true, true)
	s, h := testServer(t, demo)

	rec := do(t, h, s, "GET", "/api/state", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("состояние: код %d", rec.Code)
	}
	var st State
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if !st.LoggedIn || st.QueueLen != 4 || st.Link.State != "offline" || st.Now == nil || st.Now.Artist != "100 gecs" {
		t.Errorf("состояние: %+v", st)
	}

	if rec := do(t, h, s, "POST", "/api/queue/delete", `{"ids":[1287]}`, true); rec.Code != http.StatusOK {
		t.Fatalf("удаление: код %d, %s", rec.Code, rec.Body)
	}
	var q []QueueItem
	rec = do(t, h, s, "GET", "/api/queue", "", true)
	if err := json.Unmarshal(rec.Body.Bytes(), &q); err != nil {
		t.Fatal(err)
	}
	if len(q) != 3 {
		t.Errorf("после удаления в очереди %d записей", len(q))
	}
	for _, it := range q {
		if it.ID == 1287 {
			t.Error("удалённая запись осталась в очереди")
		}
	}

	if rec := do(t, h, s, "POST", "/api/queue/delete", `{не json`, true); rec.Code != http.StatusBadRequest {
		t.Errorf("кривой запрос: код %d", rec.Code)
	}
}

func TestНастройкиИПауза(t *testing.T) {
	demo := NewDemo(true, false)
	s, h := testServer(t, demo)

	if rec := do(t, h, s, "POST", "/api/pause", `{"paused":true}`, true); rec.Code != http.StatusOK {
		t.Fatalf("пауза: код %d", rec.Code)
	}
	if !demo.State().Paused {
		t.Error("пауза не включилась")
	}

	rec := do(t, h, s, "GET", "/api/settings", "", true)
	var set Settings
	if err := json.Unmarshal(rec.Body.Bytes(), &set); err != nil {
		t.Fatal(err)
	}
	set.Percent = 0.75
	set.Players[1].Enabled = false
	body, _ := json.Marshal(set)
	if rec := do(t, h, s, "POST", "/api/settings", string(body), true); rec.Code != http.StatusOK {
		t.Fatalf("сохранение настроек: код %d, %s", rec.Code, rec.Body)
	}
	if got := demo.Settings(); got.Percent != 0.75 || got.Players[1].Enabled {
		t.Errorf("настройки не сохранились: %+v", got)
	}

	set.Percent = 1.5
	body, _ = json.Marshal(set)
	if rec := do(t, h, s, "POST", "/api/settings", string(body), true); rec.Code != http.StatusBadRequest {
		t.Errorf("доля больше 100 %%: код %d", rec.Code)
	}
}

func TestОткрытиеТолькоИзвестныхСтраниц(t *testing.T) {
	s, h := testServer(t, NewDemo(true, false))
	if rec := do(t, h, s, "POST", "/api/open", `{"what":"profile"}`, true); rec.Code != http.StatusOK {
		t.Errorf("профиль: код %d", rec.Code)
	}
	rec := do(t, h, s, "POST", "/api/open", `{"what":"file:///C:/Windows"}`, true)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("произвольный адрес: код %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "неизвестная страница") {
		t.Errorf("ответ: %s", body)
	}
}

func TestВходВДемонстрации(t *testing.T) {
	s, h := testServer(t, NewDemo(false, false))
	if rec := do(t, h, s, "POST", "/api/keys", `{"apiKey":"коротко","secret":"x"}`, true); rec.Code != http.StatusBadRequest {
		t.Errorf("неверные ключи: код %d", rec.Code)
	}
	rec := do(t, h, s, "POST", "/api/login/start", `{}`, true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "last.fm") {
		t.Errorf("начало входа: код %d, %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, s, "POST", "/api/login/check", `{}`, true); !strings.Contains(rec.Body.String(), `"done":false`) {
		t.Errorf("сразу после начала вход не завершён: %s", rec.Body)
	}
}

func TestСтатикаВстроена(t *testing.T) {
	s, h := testServer(t, NewDemo(true, false))
	for _, path := range []string{"/static/app.css", "/static/theme.css", "/static/skin-brat.css", "/static/skin-glitch.css",
		"/static/glitch.js", "/static/app.js", "/static/icon.png",
		"/static/fonts/tiny5-cyrillic-400-normal.woff2", "/static/fonts/OFL-Tiny5.txt",
		"/static/fonts/roboto-condensed-cyrillic-wght-normal.woff2", "/static/fonts/OFL-RobotoCondensed.txt",
		"/static/fonts/martian-mono-cyrillic-standard-normal.woff2", "/static/fonts/OFL-MartianMono.txt",
		"/static/fonts/rubik-glitch-cyrillic-400-normal.woff2", "/static/fonts/OFL-RubikGlitch.txt",
		"/static/skin-fallen.css", "/static/fonts/forum-cyrillic-400-normal.woff2", "/static/fonts/OFL-Forum.txt",
		"/static/fonts/handjet-cyrillic-500-normal.woff2", "/static/fonts/OFL-Handjet.txt"} {
		if rec := do(t, h, s, "GET", path, "", false); rec.Code != http.StatusOK || rec.Body.Len() == 0 {
			t.Errorf("%s: код %d", path, rec.Code)
		}
	}
}

func TestРадиоСтанцииИВключение(t *testing.T) {
	s, h := testServer(t, NewDemo(true, false))
	if rec := do(t, h, s, "POST", "/api/radio/stations", `{"name":"","url":"ftp://x"}`, true); rec.Code != http.StatusBadRequest {
		t.Errorf("неверный адрес станции: код %d", rec.Code)
	}
	rec := do(t, h, s, "POST", "/api/radio/stations", `{"name":"Моя","url":"http://radio.example/live"}`, true)
	var st Station
	if err := json.NewDecoder(rec.Body).Decode(&st); err != nil || st.ID == "" || st.Name != "Моя" {
		t.Fatalf("добавление станции: %d %+v %v", rec.Code, st, err)
	}
	if rec := do(t, h, s, "POST", "/api/radio/play", `{"id":"`+st.ID+`"}`, true); rec.Code != http.StatusOK {
		t.Fatalf("включение: код %d", rec.Code)
	}
	var state State
	json.NewDecoder(do(t, h, s, "GET", "/api/state", "", true).Body).Decode(&state)
	if !state.Radio.Playing || state.Radio.Station != st.ID || state.Radio.Name != "Моя" {
		t.Errorf("состояние радио: %+v", state.Radio)
	}
	if rec := do(t, h, s, "POST", "/api/radio/volume", `{"volume":150}`, true); rec.Code != http.StatusBadRequest {
		t.Errorf("громкость больше 100: код %d", rec.Code)
	}
	do(t, h, s, "POST", "/api/radio/stop", `{}`, true)
	json.NewDecoder(do(t, h, s, "GET", "/api/state", "", true).Body).Decode(&state)
	if state.Radio.Playing {
		t.Error("радио должно выключиться")
	}
}

func TestПотокРадиоТолькоСКлючом(t *testing.T) {
	s, h := testServer(t, NewDemo(true, false))
	do(t, h, s, "POST", "/api/radio/play", `{"id":"1"}`, true)
	if rec := do(t, h, s, "GET", "/radio/stream?s=1", "", true); rec.Code != http.StatusUnauthorized {
		t.Errorf("поток без ключа в адресе: код %d", rec.Code)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = "127.0.0.1:" + s.port
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/radio/stream?s=1&t=" + s.token)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 4)
	if _, err := io.ReadFull(resp.Body, buf); err != nil || resp.Header.Get("Content-Type") != "audio/mpeg" ||
		buf[0] != 0xFF || buf[1] != 0xFB {
		t.Errorf("поток: %v %q % x", err, resp.Header.Get("Content-Type"), buf)
	}
}

func TestТопИКарточкиВДемонстрации(t *testing.T) {
	s, h := testServer(t, NewDemo(true, false))
	rec := do(t, h, s, "GET", "/api/stats/top?kind=tracks&period=month&offset=-1", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("топ: %d %s", rec.Code, rec.Body)
	}
	var top struct {
		Kind   string `json:"kind"`
		Period struct {
			Label   string `json:"label"`
			Live    bool   `json:"live"`
			HasNext bool   `json:"hasNext"`
		} `json:"period"`
		Items []struct {
			Rank   int    `json:"rank"`
			Artist string `json:"artist"`
			Title  string `json:"title"`
			Count  int    `json:"count"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&top)
	if top.Kind != "tracks" || len(top.Items) != 10 || top.Items[0].Rank != 1 || top.Items[0].Count < top.Items[9].Count {
		t.Errorf("топ треков: %+v", top)
	}
	if top.Period.Live || !top.Period.HasNext || top.Period.Label == "" {
		t.Errorf("прошлый месяц: %+v", top.Period)
	}

	first := top.Items[0]
	q := "/api/stats/card?kind=track&artist=" + url.QueryEscape(first.Artist) + "&title=" + url.QueryEscape(first.Title)
	rec = do(t, h, s, "GET", q, "", true)
	var card struct {
		Count int    `json:"count"`
		Album string `json:"album"`
	}
	json.NewDecoder(rec.Body).Decode(&card)
	if rec.Code != http.StatusOK || card.Count < first.Count || card.Album == "" {
		t.Errorf("карточка: %d %+v", rec.Code, card)
	}

	for _, bad := range []string{"/api/stats/top?kind=songs&period=week", "/api/stats/top?kind=tracks&period=day",
		"/api/stats/top?kind=tracks&period=week&offset=x", "/api/stats/card?kind=genre&artist=x"} {
		if rec := do(t, h, s, "GET", bad, "", true); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}
	if rec := do(t, h, s, "GET", "/api/stats/top?kind=tracks&period=week", "", false); rec.Code != http.StatusUnauthorized {
		t.Errorf("топ без ключа доступа: %d", rec.Code)
	}
	if rec := do(t, h, s, "POST", "/api/library/reload", "{}", true); rec.Code != http.StatusOK {
		t.Errorf("перезагрузка истории: %d", rec.Code)
	}
}

func TestСведенияОСборкеБезКлюча(t *testing.T) {
	s, h := testServer(t, NewDemo(true, false))
	if rec := do(t, h, s, "GET", "/about", "", false); rec.Code != http.StatusNotFound {
		t.Errorf("до SetAbout: %d", rec.Code)
	}
	s.SetAbout(map[string]string{"version": "v1", "dir": `C:\Программы`})
	rec := do(t, h, s, "GET", "/about", "", false)
	var got map[string]string
	json.NewDecoder(rec.Body).Decode(&got)
	if rec.Code != http.StatusOK || got["version"] != "v1" || got["dir"] != `C:\Программы` {
		t.Errorf("сведения: %d %v", rec.Code, got)
	}
}

func TestОбложкаОтдаётсяПоКлючуДоступа(t *testing.T) {
	s, h := testServer(t, NewDemo(true, false))
	var state State
	plain := do(t, h, s, "GET", "/api/state", "", true)
	if err := json.Unmarshal(plain.Body.Bytes(), &state); err != nil || state.Now == nil || state.Now.Cover != "" {
		t.Fatalf("обычному окну обложка не нужна: %v %+v", err, state.Now)
	}
	st := do(t, h, s, "GET", "/api/state?cover=1", "", true)
	if err := json.Unmarshal(st.Body.Bytes(), &state); err != nil || state.Now == nil || state.Now.Cover == "" {
		t.Fatalf("мини-плееру отдаётся обложка трека: %v %+v", err, state.Now)
	}

	req := httptest.NewRequest("GET", "/cover/"+state.Now.Cover+"?t="+s.token, nil)
	req.Host = "127.0.0.1:" + s.port
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || rec.Body.Len() < 100 {
		t.Fatalf("обложка: %d %q %d байт", rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
	}

	for _, c := range []struct {
		path string
		code int
	}{
		{"/cover/" + state.Now.Cover + "?t=чужой", http.StatusUnauthorized},
		{"/cover/" + state.Now.Cover, http.StatusUnauthorized},
		{"/cover/нет-такой?t=" + s.token, http.StatusNotFound},
	} {
		req := httptest.NewRequest("GET", c.path, nil)
		req.Host = "127.0.0.1:" + s.port
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.code {
			t.Errorf("%s: код %d, ожидался %d", c.path, rec.Code, c.code)
		}
	}
}

func TestКнопкаОбновитьЗапрашиваетLastfm(t *testing.T) {
	s, h := testServer(t, NewDemo(true, false))
	if rec := do(t, h, s, "POST", "/api/library/sync", `{}`, true); rec.Code != http.StatusOK {
		t.Errorf("с ключом: код %d", rec.Code)
	}
	if rec := do(t, h, s, "POST", "/api/library/sync", `{}`, false); rec.Code != http.StatusUnauthorized {
		t.Errorf("без ключа: код %d", rec.Code)
	}
}
