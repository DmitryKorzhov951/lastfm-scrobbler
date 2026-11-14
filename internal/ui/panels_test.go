package ui

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
)

func TestОкнаЗакрываютсяИОткрываются(t *testing.T) {
	d := NewDemo(true, false)
	s, h := testServer(t, d)
	if rec := do(t, h, s, "POST", "/api/panels", `{"closed":["recent","lyrics","recent"]}`, true); rec.Code != http.StatusOK {
		t.Fatalf("закрытие окон: код %d, %s", rec.Code, rec.Body)
	}
	var st State
	if rec := do(t, h, s, "GET", "/api/state", "", true); json.Unmarshal(rec.Body.Bytes(), &st) != nil || !slices.Equal(st.Closed, []string{"lyrics", "recent"}) {
		t.Errorf("закрытые окна по порядку и без повторов: %v", st.Closed)
	}
	if rec := do(t, h, s, "POST", "/api/panels", `{"closed":["settings"]}`, true); rec.Code != http.StatusBadRequest {
		t.Errorf("неизвестное окно: код %d", rec.Code)
	}
	if rec := do(t, h, s, "POST", "/api/panels", `{"closed":[]}`, false); rec.Code != http.StatusUnauthorized {
		t.Errorf("без ключа окна не меняются: код %d", rec.Code)
	}
	if rec := do(t, h, s, "POST", "/api/panels", `{"closed":[]}`, true); rec.Code != http.StatusOK {
		t.Fatalf("открытие окон: код %d", rec.Code)
	}
	if rec := do(t, h, s, "GET", "/api/state", "", true); json.Unmarshal(rec.Body.Bytes(), &st) != nil || len(st.Closed) != 0 {
		t.Errorf("все окна снова открыты: %v", st.Closed)
	}
}

func TestТекстПесниЧерезAPI(t *testing.T) {
	s, h := testServer(t, NewDemo(true, false))
	var l Lyrics
	rec := do(t, h, s, "GET", "/api/lyrics", "", true)
	if json.Unmarshal(rec.Body.Bytes(), &l) != nil || l.State != "found" || len(l.Synced) < 10 || l.Artist == "" || l.Title == "" {
		t.Fatalf("текст песни: %s", rec.Body.String())
	}
	for i := 1; i < len(l.Synced); i++ {
		if l.Synced[i].At <= l.Synced[i-1].At {
			t.Fatalf("строки по времени: %+v", l.Synced)
		}
	}
	if rec := do(t, h, s, "GET", "/api/lyrics", "", false); rec.Code != http.StatusUnauthorized {
		t.Errorf("без ключа текст не отдаётся: код %d", rec.Code)
	}
	if rec := do(t, h, s, "POST", "/api/open", `{"what":"lyrics"}`, true); rec.Code != http.StatusOK {
		t.Errorf("поиск текста в браузере: код %d", rec.Code)
	}
}

func TestРекомендацииЧерезAPI(t *testing.T) {
	d := NewDemo(true, false)
	s, h := testServer(t, d)
	var r Recs
	rec := do(t, h, s, "GET", "/api/recs", "", true)
	if json.Unmarshal(rec.Body.Bytes(), &r) != nil || r.State != RecsFound || len(r.Artists) != 5 || len(r.Tracks) != 5 || r.Title == "" {
		t.Fatalf("рекомендации: %s", rec.Body.String())
	}
	if rec := do(t, h, s, "GET", "/api/recs", "", false); rec.Code != http.StatusUnauthorized {
		t.Errorf("без ключа рекомендации не отдаются: код %d", rec.Code)
	}
	if rec := do(t, h, s, "POST", "/api/open", `{"what":"lastfm","artist":"SOPHIE","track":"Immaterial"}`, true); rec.Code != http.StatusOK {
		t.Errorf("страница трека на Last.fm: код %d, %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, s, "POST", "/api/open", `{"what":"lastfm","artist":"  "}`, true); rec.Code != http.StatusBadRequest {
		t.Errorf("без исполнителя — ошибка: код %d", rec.Code)
	}
	if rec := do(t, h, s, "POST", "/api/panels", `{"closed":["recs"]}`, true); rec.Code != http.StatusOK {
		t.Errorf("окно рекомендаций закрывается: код %d, %s", rec.Code, rec.Body)
	}
	var st State
	if rec := do(t, h, s, "GET", "/api/state", "", true); json.Unmarshal(rec.Body.Bytes(), &st) != nil || !slices.Equal(st.Closed, []string{"recs"}) {
		t.Errorf("закрытые окна: %v", st.Closed)
	}
}
