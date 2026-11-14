package main

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/ui"
)

func TestРекомендацииНовоеСначала(t *testing.T) {
	var asks atomic.Int32
	old := strconvI(time.Now().Add(-48 * time.Hour).Unix())
	svc, w, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch r.Form.Get("method") {
		case "user.getRecentTracks":
			io.WriteString(rw, `{"recenttracks":{"track":[
				{"artist":{"#text":"Знакомая группа"},"name":"Знакомая песня","date":{"uts":"`+old+`"}},
				{"artist":{"#text":"знакомая группа"},"name":"знакомая песня","date":{"uts":"`+old+`1"}}],
				"@attr":{"page":"1","totalPages":"1","total":"2"}}}`)
		case "artist.getSimilar":
			asks.Add(1)
			io.WriteString(rw, `{"similarartists":{"artist":[
				{"name":"Знакомая группа","match":"1"},
				{"name":"Новая один","match":"0.9"},
				{"name":"Новая два","match":"0.8"},
				{"name":"Новая три","match":"0.7"},
				{"name":"Новая четыре","match":"0.6"},
				{"name":"Новая пять","match":"0.5"}]}}`)
		case "track.getSimilar":
			asks.Add(1)
			io.WriteString(rw, `{"similartracks":{"track":[
				{"name":"Своя один","match":1,"artist":{"name":"Тестовая группа"}},
				{"name":"Знакомая песня","match":0.95,"artist":{"name":"Знакомая группа"}},
				{"name":"Чужая один","match":0.9,"artist":{"name":"Новая один"}},
				{"name":"Чужая два","match":0.85,"artist":{"name":"Новая один"}},
				{"name":"Чужая три","match":0.8,"artist":{"name":"Новая один"}},
				{"name":"Другая","match":0.7,"artist":{"name":"Новая два"}}]}}`)
		default:
			io.WriteString(rw, `{"nowplaying":{}}`)
		}
	})
	waitFor(t, "загрузка истории", func() bool {
		st := svc.libSync.Status()
		return st.State == "ready" && st.Plays == 2
	})
	b := &uiBackend{svc: svc}
	if r := b.Recs(); r.State != ui.RecsNone {
		t.Fatalf("пока ничего не играло: %+v", r)
	}
	listen(svc, w, playing("spotify", "Spotify", "Тестовая группа", "Песня"), 10*time.Second)
	if r := b.Recs(); r.State != ui.RecsLoading {
		t.Fatalf("сначала ищем: %+v", r)
	}
	svc.recs.busy.Wait()
	r := b.Recs()
	if r.State != ui.RecsFound || r.Artist != "Тестовая группа" || r.Title != "Песня" {
		t.Fatalf("рекомендации: %+v", r)
	}
	var names []string
	for _, a := range r.Artists {
		names = append(names, a.Name)
	}
	if strings.Join(names, ",") != "Новая один,Новая два,Новая три,Новая четыре,Новая пять" {
		t.Errorf("исполнители — сначала новые: %v", names)
	}
	var songs []string
	for _, s := range r.Tracks {
		songs = append(songs, s.Title)
	}
	if strings.Join(songs, ",") != "Чужая один,Чужая два,Другая,Своя один,Знакомая песня" || r.Tracks[4].Plays != 2 {
		t.Errorf("треки — новые у других исполнителей (не больше двух на исполнителя), потом свои, потом знакомые: %v, %+v", songs, r.Tracks)
	}
	for i := 0; i < 5; i++ {
		b.Recs()
	}
	svc.recs.busy.Wait()
	if asks.Load() != 2 {
		t.Errorf("на песню два запроса к Last.fm: %d", asks.Load())
	}

	w.ch <- media.State{ID: "spotify", Name: "spotify", Gone: true, At: time.Now()}
	barrier(svc)
	if _, ok := svc.activeSnapshot(); ok {
		t.Fatal("плеер должен пропасть")
	}
	if r := b.Recs(); r.Title != "Песня" || r.State != ui.RecsFound {
		t.Errorf("когда ничего не играет, видны последние рекомендации: %+v", r)
	}
}

func TestСсылкаНаLastfm(t *testing.T) {
	for in, want := range map[[2]string]string{
		{"Charli XCX", "360"}:          "https://www.last.fm/music/Charli+XCX/_/360",
		{"AC/DC", "Back In Black"}:     "https://www.last.fm/music/AC%2FDC/_/Back+In+Black",
		{" Земфира ", ""}:              "https://www.last.fm/music/%D0%97%D0%B5%D0%BC%D1%84%D0%B8%D1%80%D0%B0",
		{"Simon & Garfunkel", "Boxer"}: "https://www.last.fm/music/Simon+%26+Garfunkel/_/Boxer",
	} {
		if got := lastfmURL(in[0], in[1]); got != want {
			t.Errorf("lastfmURL(%q, %q) = %s, ожидалось %s", in[0], in[1], got, want)
		}
	}
	if err := (&uiBackend{}).OpenLastfm("  ", ""); err == nil {
		t.Error("без исполнителя страница не открывается")
	}
}
