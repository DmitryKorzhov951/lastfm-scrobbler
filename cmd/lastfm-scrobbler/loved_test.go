package main

import (
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestЛюбимыйТрекОтмечаетсяИПроверяетсяРедко(t *testing.T) {
	var asks atomic.Int32
	svc, w, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("method") != "track.getInfo" {
			io.WriteString(rw, `{"nowplaying":{}}`)
			return
		}
		asks.Add(1)
		if r.Form.Get("username") != "listener" {
			t.Errorf("спрашивать надо про своего слушателя: %s", r.Form.Encode())
		}
		switch r.Form.Get("track") {
		case "360":
			io.WriteString(rw, `{"track":{"name":"360","userloved":"1"}}`)
		case "Сбой":
			rw.WriteHeader(http.StatusServiceUnavailable)
		default:
			io.WriteString(rw, `{"track":{"name":"Von dutch","userloved":"0"}}`)
		}
	})
	b := &uiBackend{svc: svc}
	listen(svc, w, playing("spotify", "Spotify", "Charli XCX", "360"), 10*time.Second)
	waitFor(t, "надпись «любимка» у трека из любимых", func() bool {
		st := b.State()
		return st.Now != nil && st.Now.Loved
	})
	if asks.Load() != 1 {
		t.Errorf("запросов track.getInfo: %d, ожидался один", asks.Load())
	}
	for i := 0; i < 20; i++ {
		b.State()
	}
	svc.love.busy.Wait()
	if asks.Load() != 1 {
		t.Errorf("окно спрашивает состояние часто, но Last.fm — не чаще раза в %v: %d запросов", lovedFresh, asks.Load())
	}

	expire := func(artist, title string) {
		svc.love.mu.Lock()
		svc.love.known[lovedKey("listener", artist, title)].next = time.Now().Add(-time.Second)
		svc.love.mu.Unlock()
	}
	expire("Charli XCX", "360")
	if !svc.loved("Charli XCX", "360", false) {
		t.Error("на паузе остаётся прежняя отметка")
	}
	svc.love.busy.Wait()
	if asks.Load() != 1 {
		t.Errorf("на паузе Last.fm не спрашивается заново: %d", asks.Load())
	}
	svc.loved("charli xcx", "360 ", true)
	svc.love.busy.Wait()
	if asks.Load() != 2 {
		t.Errorf("пока трек играет, отметка перепроверяется: %d", asks.Load())
	}

	if svc.loved("Charli XCX", "Von dutch", true) {
		t.Error("до ответа Last.fm трек не считается любимым")
	}
	svc.love.busy.Wait()
	if svc.loved("Charli XCX", "Von dutch", true) {
		t.Error("обычный трек отмечен любимым")
	}

	svc.loved("Charli XCX", "Сбой", false)
	svc.love.busy.Wait()
	before := asks.Load()
	svc.loved("Charli XCX", "Сбой", false)
	svc.love.busy.Wait()
	if asks.Load() != before {
		t.Errorf("после сбоя следующая попытка не раньше чем через %v", lovedRetry)
	}
	expire("Charli XCX", "Сбой")
	svc.loved("Charli XCX", "Сбой", false)
	svc.love.busy.Wait()
	if asks.Load() != before+1 {
		t.Error("после сбоя Last.fm спрашивается снова, даже на паузе")
	}

	if err := svc.Logout(); err != nil {
		t.Fatal(err)
	}
	before = asks.Load()
	if svc.loved("Charli XCX", "360", true) {
		t.Error("без входа любимых треков нет")
	}
	svc.love.busy.Wait()
	if asks.Load() != before {
		t.Error("без входа Last.fm не спрашивается")
	}
}
