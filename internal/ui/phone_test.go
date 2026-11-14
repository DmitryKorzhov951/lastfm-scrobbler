package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testPhoneToken = "0123456789abcdef0123456789abcdef"

func phoneServer(t *testing.T) *Phone {
	t.Helper()
	p, err := ListenPhone(NewDemo(true, false), "127.0.0.1:0", testPhoneToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.ln.Close() })
	return p
}

func phoneGet(p *Phone, method, path, remote string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remote
	rec := httptest.NewRecorder()
	p.Handler().ServeHTTP(rec, req)
	return rec
}

func TestТелефонБезКлючаНичегоНеВидит(t *testing.T) {
	p := phoneServer(t)
	for _, path := range []string{
		"/p/" + strings.Repeat("0", 32) + "/api/now",
		"/p/" + strings.Repeat("0", 32) + "/",
		"/",
		"/api/state",
		"/static/app.js",
	} {
		if rec := phoneGet(p, "GET", path, "192.168.1.20:5000"); rec.Code != http.StatusNotFound {
			t.Errorf("%s: код %d, ожидался 404", path, rec.Code)
		}
	}

	rec := phoneGet(p, "GET", p.Path()+"api/now", "192.168.1.20:5000")
	var now PhoneNow
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &now) != nil || now.Now == nil || now.Now.Title == "" {
		t.Fatalf("со своим ключом виден трек: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Referrer-Policy") != "no-referrer" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("ссылка с ключом не должна утекать и кэшироваться")
	}
}

func TestТелефонТолькоЧитает(t *testing.T) {
	p := phoneServer(t)
	for _, path := range []string{"api/now", "api/history", "api/top"} {
		if rec := phoneGet(p, "POST", p.Path()+path, "192.168.1.20:5000"); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s: код %d, ожидался 405", path, rec.Code)
		}
	}
	for _, path := range []string{"api/pause", "api/settings", "api/logout", "api/queue/clear"} {
		if rec := phoneGet(p, "GET", p.Path()+path, "192.168.1.20:5000"); rec.Code != http.StatusNotFound {
			t.Errorf("%s: код %d — управлять программой с телефона нельзя", path, rec.Code)
		}
	}
}

func TestТелефонТолькоИзДомашнейСети(t *testing.T) {
	p := phoneServer(t)
	cases := map[string]int{
		"192.168.1.20:5000":    http.StatusOK,
		"10.0.0.7:5000":        http.StatusOK,
		"172.20.1.2:5000":      http.StatusOK,
		"100.101.102.103:5000": http.StatusOK,
		"[fe80::1]:5000":       http.StatusOK,
		"127.0.0.1:5000":       http.StatusOK,
		"8.8.8.8:5000":         http.StatusForbidden,
		"[2a00:1450::1]:5000":  http.StatusForbidden,
		"203.0.113.9:5000":     http.StatusForbidden,
	}
	for remote, want := range cases {
		if rec := phoneGet(p, "GET", p.Path()+"api/now", remote); rec.Code != want {
			t.Errorf("с адреса %s: код %d, ожидался %d", remote, rec.Code, want)
		}
	}
}

func TestТелефонОтдаётТолькоСвоиФайлы(t *testing.T) {
	p := phoneServer(t)
	cases := map[string]int{
		"static/phone.js":  http.StatusOK,
		"static/phone.css": http.StatusOK,
		"static/theme.css": http.StatusOK,
		"static/glitch.js": http.StatusOK,
		"static/fonts/tiny5-cyrillic-400-normal.woff2":             http.StatusOK,
		"static/fonts/roboto-condensed-cyrillic-wght-normal.woff2": http.StatusOK,
		"static/fonts/martian-mono-cyrillic-standard-normal.woff2": http.StatusOK,
		"static/fonts/rubik-glitch-cyrillic-400-normal.woff2":      http.StatusOK,
		"static/fonts/forum-cyrillic-400-normal.woff2":             http.StatusOK,
		"static/fonts/handjet-cyrillic-500-normal.woff2":           http.StatusOK,
		"static/fonts/OFL-Forum.txt":                               http.StatusNotFound,
		"static/skin-fallen.css":                                   http.StatusNotFound,
		"static/app.js":                                            http.StatusNotFound,
		"static/skin-brat.css":                                     http.StatusNotFound,
		"static/skin-glitch.css":                                   http.StatusNotFound,
		"static/index.html":                                        http.StatusNotFound,
		"static/fonts/OFL-Tiny5.txt":                               http.StatusNotFound,
	}
	for path, want := range cases {
		if rec := phoneGet(p, "GET", p.Path()+path, "192.168.1.20:5000"); rec.Code != want {
			t.Errorf("%s: код %d, ожидался %d", path, rec.Code, want)
		}
	}
	if rec := phoneGet(p, "GET", p.Path()+"static/../static/app.js", "192.168.1.20:5000"); rec.Code == http.StatusOK {
		t.Error("обходным путём чужой файл не отдаётся")
	}
	rec := phoneGet(p, "GET", p.Path(), "192.168.1.20:5000")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "static/phone.js") {
		t.Fatalf("страница телефона: %d", rec.Code)
	}
	if rec := phoneGet(p, "GET", strings.TrimSuffix(p.Path(), "/"), "192.168.1.20:5000"); rec.Code != http.StatusFound {
		t.Errorf("адрес без косой черты в конце перенаправляется: %d", rec.Code)
	}
}

func TestТелефонИсторияИТоп(t *testing.T) {
	p := phoneServer(t)
	rec := phoneGet(p, "GET", p.Path()+"api/history?limit=3", "192.168.1.20:5000")
	var list []Scrobble
	if json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list) != 3 {
		t.Fatalf("история с limit=3: %s", rec.Body.String())
	}
	rec = phoneGet(p, "GET", p.Path()+"api/history?limit=100000", "192.168.1.20:5000")
	if json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list) > PhoneHistoryMax {
		t.Fatalf("история не длиннее %d", PhoneHistoryMax)
	}
	rec = phoneGet(p, "GET", p.Path()+"api/top?period=month", "192.168.1.20:5000")
	var top struct {
		Kind  string `json:"kind"`
		Items []struct {
			Title string `json:"title"`
		} `json:"items"`
	}
	if json.Unmarshal(rec.Body.Bytes(), &top) != nil || top.Kind != "tracks" || len(top.Items) == 0 || top.Items[0].Title == "" {
		t.Fatalf("топ треков месяца: %s", rec.Body.String())
	}
}

func TestQRКодКвадратныйСПолями(t *testing.T) {
	rows, err := QRRows("http://192.168.1.20:47200/p/" + testPhoneToken + "/")
	if err != nil {
		t.Fatal(err)
	}
	n := len(rows)
	if n < 25 {
		t.Fatalf("слишком маленький код: %d", n)
	}
	for _, r := range rows {
		if len(r) != n {
			t.Fatal("код квадратный")
		}
	}
	if strings.Contains(rows[0]+rows[1]+rows[n-1], "#") {
		t.Error("по краям поле без точек")
	}
	if rows[2][2:9] != "#######" {
		t.Errorf("в углу поисковый квадрат: %q", rows[2])
	}
}

func TestПустойКлючТелефонаНеПринимается(t *testing.T) {
	if _, err := ListenPhone(NewDemo(true, false), "127.0.0.1:0", "", nil); err == nil {
		t.Fatal("без ключа страница для телефона не открывается")
	}
}

func TestМестаВТопахВОкнеИНаТелефоне(t *testing.T) {
	d := NewDemo(true, false)
	s, h := testServer(t, d)
	var st State
	if rec := do(t, h, s, "GET", "/api/state", "", true); json.Unmarshal(rec.Body.Bytes(), &st) != nil || st.Now == nil || len(st.Now.Charts) == 0 {
		t.Fatalf("у текущего трека нет мест в топах: %s", rec.Body.String())
	}
	kinds := map[string]bool{"track": true, "artist": true, "album": true}
	periods := map[string]bool{"week": true, "month": true, "year": true}
	for _, c := range st.Now.Charts {
		if !kinds[c.Kind] || !periods[c.Period] || c.Rank < 1 || c.Rank > 10 {
			t.Errorf("непонятное место: %+v", c)
		}
	}
	p, err := ListenPhone(d, "127.0.0.1:0", testPhoneToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.ln.Close() })
	var now PhoneNow
	if rec := phoneGet(p, "GET", p.Path()+"api/now", "192.168.1.20:5000"); json.Unmarshal(rec.Body.Bytes(), &now) != nil || now.Now == nil || len(now.Now.Charts) != len(st.Now.Charts) {
		t.Errorf("телефон получает те же места: %s", rec.Body.String())
	}
}

func TestЛюбимкаВОкнеИНаТелефоне(t *testing.T) {
	for _, loggedIn := range []bool{true, false} {
		d := NewDemo(loggedIn, false)
		s, h := testServer(t, d)
		var st State
		if rec := do(t, h, s, "GET", "/api/state", "", true); json.Unmarshal(rec.Body.Bytes(), &st) != nil || st.Now == nil || st.Now.Loved != loggedIn {
			t.Fatalf("вход %v: отметка «любимка» %s", loggedIn, rec.Body.String())
		}
		p, err := ListenPhone(d, "127.0.0.1:0", testPhoneToken, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { p.ln.Close() })
		var now PhoneNow
		if rec := phoneGet(p, "GET", p.Path()+"api/now", "192.168.1.20:5000"); json.Unmarshal(rec.Body.Bytes(), &now) != nil || now.Now == nil || now.Now.Loved != loggedIn {
			t.Errorf("вход %v: телефон получает ту же отметку: %s", loggedIn, rec.Body.String())
		}
	}
}
