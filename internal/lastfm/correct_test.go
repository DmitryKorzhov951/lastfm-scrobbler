package lastfm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCorrectБерётИменаСLastfm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("method") != "track.getInfo" || q.Get("artist") != "тестовая групa" || q.Get("track") != "песня" || q.Get("autocorrect") != "1" {
			t.Errorf("неожиданный запрос %s", r.URL.RawQuery)
		}
		w.Write([]byte(`{"track":{"name":"Песня","duration":"181000","artist":{"name":"Тестовая группа","url":"https://www.last.fm/music/x"}}}`))
	}))
	defer srv.Close()
	c := New("key", "secret")
	c.Endpoint = srv.URL
	artist, title, err := c.Correct(context.Background(), " тестовая групa ", "песня")
	if err != nil || artist != "Тестовая группа" || title != "Песня" {
		t.Fatalf("исправленные имена: %q, %q, %v", artist, title, err)
	}
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":6,"message":"Track not found"}`))
	})
	if artist, title, err := c.Correct(context.Background(), "Нет такого", "Нет такой"); err != nil || artist != "" || title != "" {
		t.Errorf("неизвестный трек — без исправления и без ошибки: %q, %q, %v", artist, title, err)
	}
}
