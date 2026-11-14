package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/history"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lastfm"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/queue"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newTestSubmitter(t *testing.T, handler http.HandlerFunc) (*submitter, *queue.Queue, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	q, err := queue.Open(filepath.Join(t.TempDir(), "queue.json"), 0)
	if err != nil {
		t.Fatal(err)
	}
	c := lastfm.New("KEY", "SECRET")
	c.Endpoint = srv.URL
	c.SessionKey = "SESSION"

	return newSubmitter(c, q, nil, time.Minute, testLogger()), q, srv
}

func testScrobble(title string) lastfm.Scrobble {
	return lastfm.Scrobble{
		Track:     lastfm.Track{Artist: "Артист", Title: title, Duration: 3 * time.Minute},
		Timestamp: time.Now().UTC().Add(-time.Minute),
	}
}

func TestУспешнаяОтправкаОчищаетОчередь(t *testing.T) {
	var calls int32
	sub, q, _ := newTestSubmitter(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		io.WriteString(w, `{"scrobbles":{"@attr":{"accepted":2,"ignored":0},"scrobble":[]}}`)
	})

	if err := q.Add(testScrobble("Первый")); err != nil {
		t.Fatal(err)
	}
	if err := q.Add(testScrobble("Второй")); err != nil {
		t.Fatal(err)
	}

	sub.flush(context.Background(), true)

	if q.Len() != 0 {
		t.Errorf("после успешной отправки очередь должна быть пуста, осталось %d", q.Len())
	}
	if calls != 1 {
		t.Errorf("запросов к Last.fm: %d, ожидался 1 (пакетная отправка)", calls)
	}
}

func TestВременнаяОшибкаСохраняетОчередь(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)

	sub, q, _ := newTestSubmitter(t, func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			io.WriteString(w, `{"error":16,"message":"The service is temporarily unavailable"}`)
			return
		}
		io.WriteString(w, `{"scrobbles":{"@attr":{"accepted":1,"ignored":0},"scrobble":[]}}`)
	})

	if err := q.Add(testScrobble("Песня")); err != nil {
		t.Fatal(err)
	}

	sub.flush(context.Background(), true)
	if q.Len() != 1 {
		t.Fatal("при временной ошибке скроббл должен остаться в очереди")
	}
	if got := q.Items()[0].Attempts; got != 1 {
		t.Errorf("счётчик попыток = %d, ожидалось 1", got)
	}
	if sub.nextAttempt.IsZero() {
		t.Error("после ошибки должна быть назначена пауза перед повтором")
	}

	sub.flush(context.Background(), false)
	if q.Items()[0].Attempts != 1 {
		t.Error("повтор не должен происходить раньше назначенного времени")
	}

	fail.Store(false)
	sub.flush(context.Background(), true)
	if q.Len() != 0 {
		t.Errorf("после восстановления связи очередь должна опустеть, осталось %d", q.Len())
	}
	if sub.failures != 0 {
		t.Errorf("счётчик неудач не сброшен: %d", sub.failures)
	}
}

func TestПостояннаяОшибкаНеБлокируетОчередь(t *testing.T) {
	sub, q, _ := newTestSubmitter(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"error":6,"message":"Invalid parameters"}`)
	})

	if err := q.Add(testScrobble("Кривой трек")); err != nil {
		t.Fatal(err)
	}
	sub.flush(context.Background(), true)

	if q.Len() != 0 {
		t.Error("скроббл с постоянной ошибкой должен отбрасываться, иначе он заблокирует очередь")
	}
}

func TestСтарыеСкробблыОтбрасываются(t *testing.T) {
	var called atomic.Bool
	sub, q, _ := newTestSubmitter(t, func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		io.WriteString(w, `{"scrobbles":{"@attr":{"accepted":1,"ignored":0},"scrobble":[]}}`)
	})

	old := lastfm.Scrobble{
		Track:     lastfm.Track{Artist: "A", Title: "Давно", Duration: time.Minute},
		Timestamp: time.Now().UTC().Add(-30 * 24 * time.Hour),
	}
	if err := q.Add(old); err != nil {
		t.Fatal(err)
	}
	if err := q.Add(testScrobble("Свежий")); err != nil {
		t.Fatal(err)
	}

	sub.flush(context.Background(), true)

	if !called.Load() {
		t.Fatal("свежий скроббл должен был уйти на сервер")
	}
	if q.Len() != 0 {
		t.Errorf("очередь должна опустеть, осталось %d", q.Len())
	}
}

func TestПереполнениеКаналаПишетВОчередь(t *testing.T) {
	sub, q, _ := newTestSubmitter(t, func(w http.ResponseWriter, r *http.Request) {})

	total := cap(sub.scrobbles) + 5
	for i := 0; i < total; i++ {
		sub.Scrobble(testScrobble("Трек"))
	}

	inChannel := len(sub.scrobbles)
	if inChannel+q.Len() != total {
		t.Errorf("потеряны скробблы: в канале %d, в очереди %d, всего отдано %d",
			inChannel, q.Len(), total)
	}
}

func TestBackoffРастётИОграничен(t *testing.T) {
	sub := &submitter{retry: time.Minute}
	cases := map[int]time.Duration{
		1:  time.Minute,
		2:  2 * time.Minute,
		3:  4 * time.Minute,
		4:  8 * time.Minute,
		10: 30 * time.Minute,
	}
	for failures, want := range cases {
		sub.failures = failures
		if got := sub.backoff(); got != want {
			t.Errorf("после %d неудач пауза = %s, ожидалось %s", failures, got, want)
		}
	}
}

func TestИсторияОтражаетСудьбуСкробблов(t *testing.T) {
	sub, q, _ := newTestSubmitter(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"scrobbles":{"@attr":{"accepted":1,"ignored":1},"scrobble":[
			{"track":{"#text":"Принятый"},"ignoredMessage":{"code":"0","#text":""}},
			{"track":{"#text":"Отклонённый"},"ignoredMessage":{"code":"1","#text":"Artist was ignored"}}]}}`)
	})
	hist, err := history.Open(filepath.Join(t.TempDir(), "history.json"), 0)
	if err != nil {
		t.Fatal(err)
	}
	sub.hist = hist

	sub.enqueue(testScrobble("Принятый"))
	sub.enqueue(testScrobble("Отклонённый"))
	sub.enqueue(lastfm.Scrobble{Track: lastfm.Track{Artist: "Радио", Title: "Поток"}, Timestamp: time.Now().UTC()})

	third := q.Items()[2].ID
	if _, err := q.Remove(third); err != nil {
		t.Fatal(err)
	}
	if err := hist.Delete(third); err != nil {
		t.Fatal(err)
	}

	sub.flush(context.Background(), true)
	if q.Len() != 0 {
		t.Fatalf("очередь должна опустеть, осталось %d", q.Len())
	}
	recent := hist.Recent(10)
	if len(recent) != 2 {
		t.Fatalf("в истории должно остаться два прослушивания: %+v", recent)
	}
	if recent[1].Status != history.Sent || recent[0].Status != history.Ignored {
		t.Errorf("статусы: %+v", recent)
	}
	if recent[0].Reason == "" {
		t.Error("у отклонённого скроббла должна быть причина")
	}
	if got := hist.Totals(time.Now()).AllCount; got != 1 {
		t.Errorf("в итоги входит только принятое прослушивание, получено %d", got)
	}
}

func TestБезВходаСкробблыЖдутВОчереди(t *testing.T) {
	var calls atomic.Int32
	sub, q, srv := newTestSubmitter(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		io.WriteString(w, `{"scrobbles":{"@attr":{"accepted":1,"ignored":0},"scrobble":[]}}`)
	})
	sub.SetClient(nil)
	if err := q.Add(testScrobble("До входа")); err != nil {
		t.Fatal(err)
	}
	sub.flush(context.Background(), true)
	sub.NowPlaying(lastfm.Track{Artist: "A", Title: "B"})
	sub.sendNowPlaying(context.Background(), <-sub.nowPlaying)
	if calls.Load() != 0 || q.Len() != 1 {
		t.Fatalf("без входа запросов быть не должно: запросов %d, в очереди %d", calls.Load(), q.Len())
	}
	if st := sub.Status(); st.LoggedIn {
		t.Error("без ключа сессии статус должен говорить, что вход не выполнен")
	}

	c := lastfm.New("KEY", "SECRET")
	c.Endpoint = srv.URL
	c.SessionKey = "SESSION"
	sub.SetClient(c)
	select {
	case <-sub.flushReq:
	default:
		t.Fatal("после входа должна быть запрошена отправка")
	}
	select {
	case np := <-sub.nowPlaying:
		if np.Title != "B" {
			t.Errorf("после входа повторено «сейчас играет» для %q", np.Title)
		}
	default:
		t.Error("после входа «сейчас играет» должно отправиться заново")
	}
	sub.flush(context.Background(), true)
	if q.Len() != 0 || calls.Load() != 1 {
		t.Errorf("после входа очередь должна уйти: запросов %d, осталось %d", calls.Load(), q.Len())
	}
}

func TestСтатусСвязиДляОкна(t *testing.T) {
	sub, q, _ := newTestSubmitter(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"error":9,"message":"Invalid session key"}`)
	})
	if err := q.Add(testScrobble("Песня")); err != nil {
		t.Fatal(err)
	}
	sub.flush(context.Background(), true)
	st := sub.Status()
	if !st.LoggedIn || !st.Failing || !st.AuthErr || st.LastErr == "" || st.NextTry.IsZero() {
		t.Errorf("после отказа в авторизации статус = %+v", st)
	}
}
