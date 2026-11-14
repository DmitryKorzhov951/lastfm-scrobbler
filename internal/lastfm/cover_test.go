package lastfm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoverURLБерётСамуюКрупнуюОбложкуАльбома(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("method") != "album.getInfo" || q.Get("artist") != "Charli xcx" || q.Get("album") != "BRAT" {
			t.Errorf("неожиданный запрос %s", r.URL.RawQuery)
		}
		w.Write([]byte(`{"album":{"image":[
			{"#text":"https://img/34s/a.png","size":"small"},
			{"#text":"https://img/300x300/a.png","size":"extralarge"},
			{"#text":"https://img/174s/a.png","size":"large"},
			{"#text":"","size":"mega"}]}}`))
	}))
	defer srv.Close()
	c := New("key", "secret")
	c.Endpoint = srv.URL

	u, err := c.CoverURL(context.Background(), "Charli xcx", "360", "BRAT")
	if err != nil || u != "https://img/300x300/a.png" {
		t.Fatalf("получено %q, %v", u, err)
	}
}

func TestCoverURLБезАльбомаИщетПоТреку(t *testing.T) {
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := r.URL.Query().Get("method")
		methods = append(methods, m)
		switch m {
		case "album.getInfo":
			w.Write([]byte(`{"error":6,"message":"Album not found"}`))
		case "track.getInfo":
			w.Write([]byte(`{"track":{"album":{"image":[
				{"#text":"https://img/300x300/2a96cbd8b46e442fc41c2b86b821562f.png","size":"extralarge"},
				{"#text":"https://img/174s/t.jpg","size":"large"}]}}}`))
		}
	}))
	defer srv.Close()
	c := New("key", "secret")
	c.Endpoint = srv.URL

	u, err := c.CoverURL(context.Background(), "Кино", "Группа крови", "Неизвестный альбом")
	if err != nil || u != "https://img/174s/t.jpg" {
		t.Fatalf("получено %q, %v; заглушка Last.fm не считается обложкой", u, err)
	}
	if len(methods) != 2 {
		t.Errorf("запросы: %v", methods)
	}

	methods = nil
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.URL.Query().Get("method"))
		w.Write([]byte(`{"error":6,"message":"Track not found"}`))
	})
	u, err = c.CoverURL(context.Background(), "Кто-то", "Что-то", "")
	if err != nil || u != "" || len(methods) != 1 || methods[0] != "track.getInfo" {
		t.Errorf("ненайденный трек — не ошибка: %q, %v, %v", u, err, methods)
	}
}
