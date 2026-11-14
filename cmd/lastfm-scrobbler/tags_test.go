package main

import (
	"io"
	"net/http"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestТегиПесниВСостоянии(t *testing.T) {
	var asks atomic.Int32
	svc, w, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch r.Form.Get("method") {
		case "track.getTopTags":
			asks.Add(1)
			if r.Form.Get("track") == "Редкая песня" {
				io.WriteString(rw, `{"toptags":{"tag":[]}}`)
				return
			}
			io.WriteString(rw, `{"toptags":{"tag":[{"count":100,"name":"hyperpop"},{"count":80,"name":"Electronic"},{"count":60,"name":"seen live"},{"count":50,"name":"pop"},{"count":10,"name":"dance"}]}}`)
		case "artist.getTopTags":
			asks.Add(1)
			io.WriteString(rw, `{"toptags":{"tag":[{"count":100,"name":"post-punk"},{"count":90,"name":"new wave"}]}}`)
		default:
			io.WriteString(rw, `{"nowplaying":{}}`)
		}
	})
	b := &uiBackend{svc: svc}
	listen(svc, w, playing("spotify", "Spotify", "Charli XCX", "360"), 10*time.Second)
	waitFor(t, "теги песни", func() bool {
		st := b.State()
		return st.Now != nil && slices.Equal(st.Now.Tags, []string{"hyperpop", "electronic", "pop"})
	})
	for i := 0; i < 10; i++ {
		b.State()
	}
	svc.tagged.busy.Wait()
	if asks.Load() != 1 {
		t.Errorf("теги одной песни спрашиваются один раз: %d запросов", asks.Load())
	}

	listen(svc, w, playing("spotify", "Spotify", "Группа", "Редкая песня"), 10*time.Second)
	waitFor(t, "теги исполнителя для песни без тегов", func() bool {
		st := b.State()
		return st.Now != nil && st.Now.Title == "Редкая песня" && slices.Equal(st.Now.Tags, []string{"post-punk", "new wave"})
	})
}
