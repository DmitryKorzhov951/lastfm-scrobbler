package lastfm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLovedСпрашиваетТрекСИменемСлушателя(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("method") != "track.getInfo" || q.Get("username") != "listener" || q.Get("autocorrect") != "1" {
			t.Errorf("неожиданный запрос %s", r.URL.RawQuery)
		}
		switch q.Get("track") {
		case "360":
			w.Write([]byte(`{"track":{"name":"360","userplaycount":"41","userloved":"1"}}`))
		case "Von dutch":
			w.Write([]byte(`{"track":{"name":"Von dutch","userplaycount":"3","userloved":"0"}}`))
		case "Нет такой":
			w.Write([]byte(`{"error":6,"message":"Track not found"}`))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":16,"message":"Service temporarily unavailable"}`))
		}
	}))
	defer srv.Close()
	c := New("key", "secret")
	c.Endpoint = srv.URL
	ctx := context.Background()

	if loved, err := c.Loved(ctx, "listener", "Charli xcx", "360"); err != nil || !loved {
		t.Errorf("любимый трек: %v, %v", loved, err)
	}
	if loved, err := c.Loved(ctx, "listener", "Charli xcx", "Von dutch"); err != nil || loved {
		t.Errorf("обычный трек: %v, %v", loved, err)
	}
	if loved, err := c.Loved(ctx, "listener", "Кино", "Нет такой"); err != nil || loved {
		t.Errorf("трека нет в Last.fm — не любимый и не ошибка: %v, %v", loved, err)
	}
	if _, err := c.Loved(ctx, "listener", "Кино", "Сбой"); err == nil {
		t.Error("сбой Last.fm должен вернуть ошибку, чтобы проверить позже")
	}
	if loved, err := c.Loved(ctx, "", "Charli xcx", "360"); err != nil || loved {
		t.Errorf("без имени слушателя ничего не спрашивается: %v, %v", loved, err)
	}
}
