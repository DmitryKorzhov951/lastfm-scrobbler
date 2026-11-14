package lastfm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTopTagsБерётТриСамыхПопулярных(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("method") != "track.getTopTags" || q.Get("artist") != "Тестовая группа" || q.Get("track") != "Песня" || q.Get("autocorrect") != "1" {
			t.Errorf("неожиданный запрос %s", r.URL.RawQuery)
		}
		w.Write([]byte(`{"toptags":{"tag":[
			{"count":100,"name":"Тестовая Группа"},
			{"count":90,"name":"Hyperpop"},
			{"count":70,"name":"seen live"},
			{"count":60,"name":"electronic"},
			{"count":55,"name":"hyperpop"},
			{"count":40,"name":"  experimental  "},
			{"count":10,"name":"pop"}
		],"@attr":{"artist":"Тестовая группа","track":"Песня"}}}`))
	}))
	defer srv.Close()
	c := New("key", "secret")
	c.Endpoint = srv.URL
	tags, err := c.TopTags(context.Background(), "Тестовая группа", "Песня", 3)
	if err != nil || len(tags) != 3 || tags[0] != "hyperpop" || tags[1] != "electronic" || tags[2] != "experimental" {
		t.Fatalf("теги: %q, %v; без имени исполнителя, «seen live» и повторов", tags, err)
	}
}

func TestTopTagsБезТеговПесниБерётТегиИсполнителя(t *testing.T) {
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := r.URL.Query().Get("method")
		methods = append(methods, m)
		switch m {
		case "track.getTopTags":
			w.Write([]byte(`{"toptags":{"tag":[],"@attr":{"artist":"Группа","track":"Редкая песня"}}}`))
		case "artist.getTopTags":
			w.Write([]byte(`{"toptags":{"tag":{"count":100,"name":"post-punk"},"@attr":{"artist":"Группа"}}}`))
		}
	}))
	defer srv.Close()
	c := New("key", "secret")
	c.Endpoint = srv.URL
	tags, err := c.TopTags(context.Background(), "Группа", "Редкая песня", 3)
	if err != nil || len(tags) != 1 || tags[0] != "post-punk" || len(methods) != 2 {
		t.Fatalf("теги исполнителя (и одиночный тег объектом): %q, %v, запросы %v", tags, err, methods)
	}

	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":6,"message":"Track not found"}`))
	})
	if tags, err := c.TopTags(context.Background(), "Нет такого", "Нет такой", 3); err != nil || len(tags) != 0 {
		t.Errorf("неизвестный трек — без тегов и без ошибки: %q, %v", tags, err)
	}
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":16,"message":"Service temporarily unavailable"}`))
	})
	if _, err := c.TopTags(context.Background(), "Группа", "Песня", 3); err == nil {
		t.Error("сбой Last.fm должен вернуть ошибку, чтобы спросить позже")
	}
}
