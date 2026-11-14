package lastfm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestПохожиеИсполнителиИТреки(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("autocorrect") != "1" || q.Get("limit") == "" {
			t.Errorf("неожиданный запрос %s", r.URL.RawQuery)
		}
		switch q.Get("method") {
		case "artist.getSimilar":
			w.Write([]byte(`{"similarartists":{"artist":[
				{"name":"Первая группа","match":"1"},
				{"name":"Вторая группа","match":"0.731"},
				{"name":"  ","match":"0.5"}
			],"@attr":{"artist":"Тестовая группа"}}}`))
		case "track.getSimilar":
			w.Write([]byte(`{"similartracks":{"track":{"name":"Похожая песня","match":0.94,"artist":{"name":"Первая группа"}},"@attr":{"artist":"Тестовая группа"}}}`))
		}
	}))
	defer srv.Close()
	c := New("key", "secret")
	c.Endpoint = srv.URL
	ctx := context.Background()

	artists, err := c.SimilarArtists(ctx, "Тестовая группа", 30)
	if err != nil || len(artists) != 2 || artists[0].Name != "Первая группа" || artists[0].Match != 1 || artists[1].Match != 0.731 {
		t.Fatalf("похожие исполнители: %+v, %v", artists, err)
	}
	tracks, err := c.SimilarTracks(ctx, "Тестовая группа", "Песня", 30)
	if err != nil || len(tracks) != 1 || tracks[0].Title != "Похожая песня" || tracks[0].Artist != "Первая группа" || tracks[0].Match != 0.94 {
		t.Fatalf("похожие треки (одиночный ответ объектом): %+v, %v", tracks, err)
	}

	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":6,"message":"The artist you supplied could not be found"}`))
	})
	if list, err := c.SimilarArtists(ctx, "Нет такого", 30); err != nil || len(list) != 0 {
		t.Errorf("неизвестный исполнитель — пусто и без ошибки: %+v, %v", list, err)
	}
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":16,"message":"Service temporarily unavailable"}`))
	})
	if _, err := c.SimilarTracks(ctx, "Тестовая группа", "Песня", 30); err == nil {
		t.Error("сбой Last.fm — ошибка, чтобы спросить позже")
	}
}
