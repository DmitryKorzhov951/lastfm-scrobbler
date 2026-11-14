package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/history"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lastfm"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/library"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/player"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/scrobbler"
)

type fakeWatcher struct{ ch chan media.State }

func (w *fakeWatcher) Events() <-chan media.State { return w.ch }
func (w *fakeWatcher) Close() error               { return nil }
func (w *fakeWatcher) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func testService(t *testing.T, handler http.HandlerFunc) (*service, *fakeWatcher, context.CancelFunc) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(dir, "config"))
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "state"))
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("LASTFM_API_ENDPOINT", srv.URL)

	cfg := config.Default()
	cfg.APIKey, cfg.APISecret = "0123456789abcdef0123456789abcdef", "fedcba9876543210fedcba9876543210"
	queuePath, err := config.QueuePath()
	if err != nil {
		t.Fatal(err)
	}
	e := &env{
		cfg:         cfg,
		cfgPath:     filepath.Join(dir, "config.json"),
		sessionPath: filepath.Join(dir, "session.json"),
		queuePath:   queuePath,
		session:     config.Session{Username: "listener", Key: "SESSION"},
		log:         testLogger(),
	}
	svc, err := newService(e)
	if err != nil {
		t.Fatal(err)
	}
	w := &fakeWatcher{ch: make(chan media.State)}
	svc.newWatcher = func(player.Options) (player.Watcher, error) { return w, nil }

	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx, func() { close(ready) }) }()
	<-ready
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return svc, w, cancel
}

func playing(name, identity, artist, title string) media.State {
	return media.State{
		ID: name, Name: name, Identity: identity, Status: media.Playing,
		Track:         media.Track{Title: title, Artists: []string{artist}, Length: time.Minute},
		At:            time.Now(),
		PositionKnown: true,
	}
}

func listen(svc *service, w *fakeWatcher, st media.State, d time.Duration) {
	start := st.At
	for t := time.Duration(0); t <= d; t += 5 * time.Second {
		s := st
		s.At, s.Position = start.Add(t), t
		w.ch <- s
	}
	barrier(svc)
}

func barrier(svc *service) {
	svc.do(context.Background(), func(*scrobbler.Engine) {})
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("не дождались: %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestСервисЗасчитываетИОтправляет(t *testing.T) {
	var scrobbles atomic.Int32
	svc, w, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("method") == "track.scrobble" {
			scrobbles.Add(1)
			io.WriteString(rw, `{"scrobbles":{"@attr":{"accepted":1,"ignored":0},"scrobble":[]}}`)
			return
		}
		io.WriteString(rw, `{"nowplaying":{}}`)
	})

	listen(svc, w, playing("spotify", "Spotify", "Charli XCX", "360"), 40*time.Second)

	snap, ok := svc.activeSnapshot()
	if !ok || snap.Identity != "Spotify" || snap.Track.Title != "360" || !snap.Scrobbld {
		t.Fatalf("снимок текущего трека: %+v, %v", snap, ok)
	}
	waitFor(t, "отправка скроббла", func() bool {
		r := svc.hist.Recent(1)
		return len(r) == 1 && r[0].Status == history.Sent
	})
	if scrobbles.Load() != 1 || svc.queue.Len() != 0 {
		t.Errorf("запросов track.scrobble %d, в очереди %d", scrobbles.Load(), svc.queue.Len())
	}
	if tot := svc.hist.Totals(time.Now()); tot.TodayCount != 1 || tot.TodayTime != time.Minute {
		t.Errorf("итоги за сегодня: %+v", tot)
	}
}

func TestСервисПаузаИВыключенныйПлеер(t *testing.T) {
	svc, w, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) {
		io.WriteString(rw, `{"scrobbles":{"@attr":{"accepted":1,"ignored":0},"scrobble":[]}}`)
	})

	svc.SetPaused(true)
	listen(svc, w, playing("spotify", "Spotify", "Lana Del Rey", "Summertime Sadness"), 40*time.Second)
	if len(svc.sub.scrobbles) != 0 || svc.queue.Len() != 0 || len(svc.hist.Recent(5)) != 0 {
		t.Fatal("на паузе прослушивание засчитываться не должно")
	}
	svc.SetPaused(false)

	cfg := svc.config()
	cfg.IgnoredPlayers = []string{"AIMP"}
	if err := svc.ApplyConfig(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	listen(svc, w, playing("aimp", "AIMP", "MGK", "bloody valentine"), 40*time.Second)
	if len(svc.sub.scrobbles) != 0 || svc.queue.Len() != 0 {
		t.Fatal("выключенный плеер не должен засчитываться")
	}
	found := false
	for _, p := range svc.hist.Players() {
		found = found || p.Name == "aimp"
	}
	if !found {
		t.Error("выключенный плеер должен оставаться в списке замеченных")
	}
	saved, err := config.Load(svc.cfgPath)
	if err != nil || len(saved.IgnoredPlayers) != 1 {
		t.Errorf("настройки не сохранились: %+v, %v", saved.IgnoredPlayers, err)
	}
}

func TestСервисУдалениеИзОчереди(t *testing.T) {
	svc, _, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusServiceUnavailable)
	})
	svc.sub.enqueue(lastfm.Scrobble{Track: lastfm.Track{Artist: "Ninajirachi", Title: "All I Am", Duration: 3 * time.Minute}, Timestamp: time.Now().UTC()})
	svc.sub.enqueue(lastfm.Scrobble{Track: lastfm.Track{Artist: "Charli XCX", Title: "Von dutch", Duration: 2 * time.Minute}, Timestamp: time.Now().UTC()})
	first := svc.queue.Items()[0].ID

	if err := svc.DeleteQueued(first); err != nil {
		t.Fatal(err)
	}
	if svc.queue.Len() != 1 || len(svc.hist.Recent(5)) != 1 {
		t.Fatalf("после удаления: очередь %d, история %d", svc.queue.Len(), len(svc.hist.Recent(5)))
	}
	if err := svc.ClearQueue(); err != nil {
		t.Fatal(err)
	}
	if svc.queue.Len() != 0 || svc.hist.Totals(time.Now()).AllCount != 0 {
		t.Error("после очистки очереди не должно остаться ни записей, ни итогов")
	}
}

func TestКлючиПриложенияПроверяются(t *testing.T) {
	svc, _, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) {})
	if err := svc.SaveKeys("коротко", "0123456789abcdef0123456789abcdef"); err == nil {
		t.Error("короткий ключ должен отвергаться")
	}
	if err := svc.SaveKeys(" 0123456789ABCDEF0123456789abcdef ", "fedcba9876543210fedcba9876543210"); err != nil {
		t.Errorf("правильный ключ с пробелами по краям: %v", err)
	}
	saved, err := config.Load(svc.cfgPath)
	if err != nil || saved.APIKey != "0123456789ABCDEF0123456789abcdef" {
		t.Errorf("ключ не сохранён: %q, %v", saved.APIKey, err)
	}
}

func TestВходЧерезБраузер(t *testing.T) {
	var approved atomic.Bool
	svc, _, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch r.Form.Get("method") {
		case "auth.getToken":
			io.WriteString(rw, `{"token":"TOKEN"}`)
		case "auth.getSession":
			if !approved.Load() {
				io.WriteString(rw, `{"error":14,"message":"Unauthorized Token - This token has not been authorized"}`)
				return
			}
			io.WriteString(rw, `{"session":{"name":"listener2","key":"NEWKEY","subscriber":0}}`)
		}
	})
	if err := svc.Logout(); err != nil {
		t.Fatal(err)
	}
	if svc.sub.Status().LoggedIn {
		t.Fatal("после выхода сессии быть не должно")
	}

	url, err := svc.StartLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := "token=TOKEN"; !strings.Contains(url, want) {
		t.Errorf("адрес подтверждения %q не содержит %q", url, want)
	}
	if _, err := svc.CheckLogin(context.Background()); err != errLoginPending {
		t.Fatalf("до подтверждения ожидалось errLoginPending, получено %v", err)
	}
	approved.Store(true)
	sess, err := svc.CheckLogin(context.Background())
	if err != nil || sess.Username != "listener2" {
		t.Fatalf("после подтверждения: %+v, %v", sess, err)
	}
	if !svc.sub.Status().LoggedIn || svc.account().Username != "listener2" {
		t.Error("сессия не применилась")
	}
	saved, err := config.LoadSession(svc.sessionPath)
	if err != nil || saved.Key != "NEWKEY" {
		t.Errorf("сессия не сохранена: %+v, %v", saved, err)
	}
}

func TestИсторияИзLastfmСчитаетсяВместеСоСвоимиПрослушиваниями(t *testing.T) {
	old1, old2 := time.Now().Add(-48*time.Hour).Unix(), time.Now().Add(-47*time.Hour).Unix()
	var users atomic.Value
	svc, w, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch r.Form.Get("method") {
		case "user.getRecentTracks":
			users.Store(r.Form.Get("user"))
			io.WriteString(rw, `{"recenttracks":{"track":[
				{"artist":{"#text":"Кино"},"album":{"#text":"Группа крови"},"name":"Группа крови","date":{"uts":"`+
				strconvI(old2)+`"}},
				{"artist":{"#text":"Кино"},"album":{"#text":"Группа крови"},"name":"Группа крови","date":{"uts":"`+
				strconvI(old1)+`"}}],"@attr":{"page":"1","totalPages":"1","total":"2"}}}`)
		case "track.scrobble":
			io.WriteString(rw, `{"scrobbles":{"@attr":{"accepted":1,"ignored":0},"scrobble":[]}}`)
		default:
			io.WriteString(rw, `{"nowplaying":{}}`)
		}
	})
	waitFor(t, "загрузка истории", func() bool {
		st := svc.libSync.Status()
		return st.State == "ready" && st.Plays == 2
	})
	if u, _ := users.Load().(string); u != "listener" {
		t.Errorf("история запрошена для %q", u)
	}

	listen(svc, w, playing("spotify", "Spotify", "Кино", "Группа крови"), 40*time.Second)
	waitFor(t, "отправка скроббла", func() bool {
		r := svc.hist.Recent(1)
		return len(r) == 1 && r[0].Status == history.Sent
	})

	b := &uiBackend{svc: svc}
	mine := svc.hist.Recent(1)[0]
	recent := b.State().Recent
	if len(recent) != 3 || recent[0].Status != "sent" || recent[1].Status != "lastfm" || recent[2].Status != "lastfm" ||
		!recent[1].Started.Equal(time.Unix(old2, 0)) {
		t.Fatalf("недавние вместе с Last.fm: %+v", recent)
	}
	card, err := b.Card("track", "кино", "группа крови", "")
	if err != nil {
		t.Fatal(err)
	}
	if card.Count != 3 || card.Album != "Группа крови" {
		t.Errorf("карточка трека: %+v", card)
	}
	if st := b.State(); st.Library.State != "ready" || st.Library.Plays != 2 {
		t.Errorf("состояние истории в окне: %+v", st.Library)
	}
	if _, err := svc.lib.Add([]library.Play{{At: mine.Started, Artist: mine.Artist, Title: mine.Title}}); err != nil {
		t.Fatal(err)
	}
	if recent := b.History(10); len(recent) != 3 || recent[0].Status != "sent" || recent[0].ID != mine.ID {
		t.Errorf("свой скроббл, пришедший и из Last.fm, показан один раз: %+v", recent)
	}
	if recent := b.State().Recent; len(recent) != 3 {
		t.Errorf("без повторов в окне: %+v", recent)
	}

	if err := svc.Logout(); err != nil {
		t.Fatal(err)
	}
	if st := svc.libSync.Status(); st.State != "off" {
		t.Errorf("после выхода: %+v", st)
	}
	if err := b.ReloadLibrary(); err == nil {
		t.Error("перезагрузка истории без входа должна отказывать")
	}
}

func strconvI(n int64) string { return strconv.FormatInt(n, 10) }

func TestТемаСохраняетсяВНастройках(t *testing.T) {
	svc, _, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) {})
	b := &uiBackend{svc: svc}
	if got := b.State().Theme; got != "standard" {
		t.Errorf("тема по умолчанию: %q", got)
	}
	s := b.Settings()
	s.Theme = "brat"
	if err := b.SaveSettings(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	saved, err := config.Load(svc.cfgPath)
	if err != nil || saved.Theme != "brat" {
		t.Errorf("тема в config.json: %q, %v", saved.Theme, err)
	}
	if b.State().Theme != "brat" || b.Settings().Theme != "brat" {
		t.Error("окно видит сохранённую тему")
	}
	s.Theme = ""
	if err := b.SaveSettings(context.Background(), s); err != nil || b.Settings().Theme != "brat" {
		t.Errorf("настройки без темы её не сбрасывают: %q, %v", b.Settings().Theme, err)
	}

	if !b.State().Bursts || b.Settings().Bursts == nil || !*b.Settings().Bursts {
		t.Error("рывки по умолчанию включены")
	}
	off := false
	s.Bursts = &off
	if err := b.SaveSettings(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if saved, err := config.Load(svc.cfgPath); err != nil || saved.GlitchBursts || b.State().Bursts {
		t.Errorf("выключенные рывки в config.json: %v, %v", saved.GlitchBursts, err)
	}
	s.Bursts = nil
	if err := b.SaveSettings(context.Background(), s); err != nil || b.State().Bursts {
		t.Errorf("настройки без флажка рывков его не трогают: %v, %v", b.State().Bursts, err)
	}
}

func TestБезВходаПрослушиванияХранятсяЛокально(t *testing.T) {
	var scrobbles atomic.Int32
	svc, w, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("method") == "track.scrobble" {
			scrobbles.Add(1)
			io.WriteString(rw, `{"scrobbles":{"@attr":{"accepted":1,"ignored":0},"scrobble":[]}}`)
			return
		}
		io.WriteString(rw, `{"nowplaying":{}}`)
	})
	if err := svc.Logout(); err != nil {
		t.Fatal(err)
	}
	old := history.Entry{ID: svc.queue.NextID(), Artist: "Кино", Title: "Кукушка", Seconds: 400,
		Started: time.Now().Add(-20 * 24 * time.Hour), Status: history.Local}
	if err := svc.hist.Add(old); err != nil {
		t.Fatal(err)
	}

	listen(svc, w, playing("spotify", "Spotify", "Charli XCX", "360"), 40*time.Second)
	waitFor(t, "запись прослушивания", func() bool {
		r := svc.hist.Recent(1)
		return len(r) == 1 && r[0].Title == "360"
	})
	mine := svc.hist.Recent(1)[0]
	if mine.Status != history.Local || svc.queue.Len() != 0 || scrobbles.Load() != 0 {
		t.Fatalf("без входа: статус %q, в очереди %d, отправок %d", mine.Status, svc.queue.Len(), scrobbles.Load())
	}

	b := &uiBackend{svc: svc}
	st := b.State()
	if st.LoggedIn || st.QueueLen != 0 || len(st.Recent) != 2 || st.Recent[0].Status != "local" {
		t.Errorf("состояние без входа: вход %v, очередь %d, недавние %+v", st.LoggedIn, st.QueueLen, st.Recent)
	}
	if st.All.Count != 1 {
		t.Errorf("всего на компьютере: %d", st.All.Count)
	}
	top, err := b.Top("tracks", "week", 0)
	if err != nil || len(top.Items) != 1 || top.Items[0].Title != "360" {
		t.Errorf("топ без входа: %+v, %v", top.Items, err)
	}

	restarted, err := library.Open(filepath.Join(os.Getenv("LOCALAPPDATA"), "lastfm-scrobbler", "local.jsonl"), "")
	if err != nil || restarted.Len() != 1 {
		t.Errorf("прослушивания на диске после перезапуска: %d, %v", restarted.Len(), err)
	}

	svc.mu.Lock()
	svc.session = config.Session{Username: "listener", Key: "SESSION"}
	cfg := svc.cfg
	svc.mu.Unlock()
	svc.sub.SetClient(svc.newClient(cfg, "SESSION"))
	svc.sendRecentLocal()
	waitFor(t, "отправка после входа", func() bool {
		return len(svc.hist.Pick(mine.ID)) == 1 && svc.hist.Pick(mine.ID)[0].Status == history.Sent
	})
	if got := svc.hist.Pick(old.ID); len(got) != 1 || got[0].Status != history.Local {
		t.Errorf("прослушивание старше 14 дней остаётся только на компьютере: %+v", got)
	}
	if scrobbles.Load() != 1 {
		t.Errorf("отправок после входа: %d", scrobbles.Load())
	}
}

func TestВыходСохраняетОчередьЛокальноАУдалениеЧиститАрхив(t *testing.T) {
	svc, w, _ := testService(t, func(rw http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("method") == "track.scrobble" {
			rw.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		io.WriteString(rw, `{"nowplaying":{}}`)
	})
	listen(svc, w, playing("spotify", "Spotify", "Земфира", "Хочешь?"), 40*time.Second)
	waitFor(t, "скроббл в очереди", func() bool { return svc.queue.Len() == 1 })
	if svc.archive.Len() != 1 {
		t.Fatalf("в архиве: %d", svc.archive.Len())
	}
	id := svc.queue.Items()[0].ID
	if err := svc.DeleteQueued(id); err != nil {
		t.Fatal(err)
	}
	if svc.archive.Len() != 0 || len(svc.hist.Pick(id)) != 0 {
		t.Errorf("удалённая запись осталась: архив %d, история %d", svc.archive.Len(), len(svc.hist.Pick(id)))
	}

	listen(svc, w, playing("spotify", "Spotify", "Земфира", "Искала"), 40*time.Second)
	waitFor(t, "второй скроббл в очереди", func() bool { return svc.queue.Len() == 1 })
	id = svc.queue.Items()[0].ID
	if err := svc.Logout(); err != nil {
		t.Fatal(err)
	}
	if got := svc.hist.Pick(id); svc.queue.Len() != 0 || len(got) != 1 || got[0].Status != history.Local {
		t.Errorf("после выхода очередь сохранена на компьютере: очередь %d, запись %+v", svc.queue.Len(), got)
	}
}
