package radio

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

func silentFrame() []byte {
	f := make([]byte, 417)
	copy(f, []byte{0xFF, 0xFB, 0x90, 0xC0})
	return f
}

func icyBody(metaint int, titles []string) []byte {
	audio := bytes.Repeat(silentFrame(), (metaint*len(titles))/417+2)
	var out bytes.Buffer
	for i, t := range titles {
		out.Write(audio[i*metaint : (i+1)*metaint])
		if t == "" {
			out.WriteByte(0)
			continue
		}
		meta := []byte("StreamTitle='" + t + "';StreamUrl='';")
		n := (len(meta) + 15) / 16
		out.WriteByte(byte(n))
		out.Write(meta)
		out.Write(make([]byte, n*16-len(meta)))
	}
	return out.Bytes()
}

func TestParseStreamTitle(t *testing.T) {
	cases := map[string]string{
		"StreamTitle='Кино - Кукушка';StreamUrl='';":                    "Кино - Кукушка",
		"StreamTitle='Guns N' Roses - Don't Cry';StreamUrl='http://x';": "Guns N' Roses - Don't Cry",
		"StreamTitle='Lana Del Rey - Young and Beautiful';\x00\x00\x00": "Lana Del Rey - Young and Beautiful",
		"StreamTitle='';":                                     "",
		"StreamTitle='MGK - my ex's best friend'":             "MGK - my ex's best friend",
		"StreamUrl='x';StreamTitle='Ninajirachi - All I Am';": "Ninajirachi - All I Am",
	}
	for in, want := range cases {
		got, ok := parseStreamTitle([]byte(in))
		if !ok || got != want {
			t.Errorf("%q → %q, %v; ждали %q", in, got, ok, want)
		}
	}
	if _, ok := parseStreamTitle([]byte("StreamUrl='x';")); ok {
		t.Error("без StreamTitle названия нет")
	}
}

func TestНазваниеВWindows1251(t *testing.T) {
	cp := []byte{0xCA, 0xE8, 0xED, 0xEE, ' ', '-', ' ', 0xCA, 0xF3, 0xEA, 0xF3, 0xF8, 0xEA, 0xE0}
	got, _ := parseStreamTitle(append(append([]byte("StreamTitle='"), cp...), "';"...))
	if got != "Кино - Кукушка" {
		t.Errorf("Windows-1251: %q", got)
	}
	got, _ = parseStreamTitle([]byte("StreamTitle='Beyonc\xe9 - Halo';"))
	if got != "Beyoncé - Halo" {
		t.Errorf("Windows-1252: %q", got)
	}
}

func TestSplitTitle(t *testing.T) {
	cases := []struct{ in, artist, title string }{
		{"Кино - Кукушка", "Кино", "Кукушка"},
		{"Charli XCX – 360", "Charli XCX", "360"},
		{"100 gecs - ringtone - remix", "100 gecs", "ringtone - remix"},
		{"Radio Record", "", "Radio Record"},
		{" - без исполнителя", "", "- без исполнителя"},
	}
	for _, c := range cases {
		a, ti, ok := SplitTitle(c.in)
		if a != c.artist || ti != c.title || ok != (c.artist != "") {
			t.Errorf("%q → %q / %q / %v", c.in, a, ti, ok)
		}
	}
}

func TestMetaReaderВырезаетВставки(t *testing.T) {
	body := icyBody(1000, []string{"A - 1", "", "A - 1", "B - 2"})
	var titles []string
	r := newMetaReader(bytes.NewReader(body), 1000, func(s string) { titles = append(titles, s) })
	audio, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(audio) != 4000 {
		t.Errorf("звука должно остаться 4000 байт, осталось %d", len(audio))
	}
	if bytes.Contains(audio, []byte("StreamTitle")) {
		t.Error("вставки с названием попали в звук")
	}
	if strings.Join(titles, "|") != "A - 1|B - 2" {
		t.Errorf("названия: %q — повтор не должен сообщаться", titles)
	}
}

func icyServer(t *testing.T, metaint int, titles []string) *httptest.Server {
	t.Helper()
	body := icyBody(metaint, titles)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Icy-MetaData") != "1" {
			t.Error("клиент должен просить названия: Icy-MetaData: 1")
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("icy-metaint", fmt.Sprint(metaint))
		w.Header().Set("icy-br", "128")
		w.Header().Set("icy-name", "Тестовое радио")
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestConnectЧитаетФорматИНазвания(t *testing.T) {
	srv := icyServer(t, 2000, []string{"Кино - Кукушка", "", "MGK - bloody valentine"})
	var titles []string
	s, err := Connect(context.Background(), NewClient(), srv.URL+"/live", "test", func(x string) { titles = append(titles, x) })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	audio, _ := io.ReadAll(s)
	if len(audio) != 6000 || s.ContentType != "audio/mpeg" || s.Format() != "MP3 · 128 кбит/с" || s.Name != "Тестовое радио" {
		t.Errorf("звук %d байт, %q, %q, %q", len(audio), s.ContentType, s.Format(), s.Name)
	}
	if len(titles) != 2 || titles[1] != "MGK - bloody valentine" {
		t.Errorf("названия: %q", titles)
	}
}

func TestConnectПонимаетОтветICY(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		bufio.NewReader(c).ReadString('\n')
		fmt.Fprint(c, "ICY 200 OK\r\nicy-name:Old Radio\r\ncontent-type:audio/mpeg\r\nicy-metaint:500\r\n\r\n")
		c.Write(icyBody(500, []string{"Old - Song"}))
	}()
	var title string
	s, err := Connect(context.Background(), NewClient(), "http://"+ln.Addr().String()+"/", "test", func(x string) { title = x })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	io.ReadAll(s)
	if s.Codec != "MP3" || title != "Old - Song" || s.Name != "Old Radio" {
		t.Errorf("ответ ICY: %q %q %q", s.Codec, title, s.Name)
	}
}

func TestConnectИдётПоПлейлистам(t *testing.T) {
	station := icyServer(t, 1000, []string{"X - Y"})
	mux := http.NewServeMux()
	mux.HandleFunc("/radio.pls", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/x-scpls")
		fmt.Fprintf(w, "[playlist]\nNumberOfEntries=1\nFile1=%s/live?x=1\nTitle1=Радио\n", station.URL)
	})
	mux.HandleFunc("/radio.m3u", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "#EXTM3U\n#EXTINF:-1,Радио\nradio.pls\n")
	})
	mux.HandleFunc("/hls.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:10\nseg1.aac\n")
	})
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<!doctype html><title>Радио</title>")
	})
	pl := httptest.NewServer(mux)
	defer pl.Close()

	s, err := Connect(context.Background(), NewClient(), pl.URL+"/radio.m3u", "test", func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if s.Codec != "MP3" {
		t.Errorf("плейлист M3U → PLS → поток: %q", s.Codec)
	}
	if _, err := Connect(context.Background(), NewClient(), pl.URL+"/hls.m3u8", "test", nil); err != ErrHLS {
		t.Errorf("HLS: %v", err)
	}
	if _, err := Connect(context.Background(), NewClient(), pl.URL+"/page", "test", nil); err != errWebPage {
		t.Errorf("веб-страница: %v", err)
	}
	if _, err := Connect(context.Background(), NewClient(), pl.URL+"/missing", "test", nil); err == nil ||
		!strings.Contains(err.Error(), "404") {
		t.Errorf("404: %v", err)
	}
}

func TestDetectПоПервымБайтам(t *testing.T) {
	cases := []struct {
		head  []byte
		codec string
	}{
		{[]byte("ID3\x04"), "MP3"},
		{[]byte{0xFF, 0xFB, 0x90, 0xC0}, "MP3"},
		{[]byte{0xFF, 0xF1, 0x50, 0x80}, "AAC"},
		{[]byte("OggS\x00"), "Ogg"},
		{[]byte("fLaC"), "FLAC"},
	}
	for _, c := range cases {
		codec, _, err := detect("application/octet-stream", c.head)
		if err != nil || codec != c.codec {
			t.Errorf("% x → %q, %v", c.head, codec, err)
		}
	}
	if codec, ct, _ := detect("audio/aacp", nil); codec != "AAC" || ct != "audio/aac" {
		t.Errorf("audio/aacp → %q %q", codec, ct)
	}
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestСтанцииСохраняются(t *testing.T) {
	path := filepath.Join(t.TempDir(), "radio.json")
	r, err := Open(path, quiet(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Add("", "ftp://example.com/x"); err == nil {
		t.Error("адрес не http должен отвергаться")
	}
	if _, err := r.Add("", "   "); err == nil {
		t.Error("пустой адрес должен отвергаться")
	}
	a, err := r.Add("  Радио   Рекорд ", "https://radiorecord.example/rr_main96.aacp")
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Add("", "http://stream.example:8000/live")
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "Радио Рекорд" || b.Name != "stream.example" {
		t.Errorf("названия: %q, %q", a.Name, b.Name)
	}
	if err := r.Edit(b.ID, "Своя станция", b.URL); err != nil {
		t.Fatal(err)
	}
	if err := r.SetVolume(150); err != nil {
		t.Fatal(err)
	}
	if err := r.Play(a.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(a.ID); err != nil {
		t.Fatal(err)
	}
	if r.Status().Playing {
		t.Error("удалённая станция должна выключиться")
	}

	r2, err := Open(path, quiet(), "test")
	if err != nil {
		t.Fatal(err)
	}
	list := r2.Stations()
	if len(list) != 1 || list[0].Name != "Своя станция" || r2.Status().Volume != 100 {
		t.Errorf("после перезапуска: %+v, громкость %d", list, r2.Status().Volume)
	}
	if last, ok := r2.LastStation(); !ok || last.ID != b.ID {
		t.Errorf("последняя станция: %+v", last)
	}
}

func TestИспорченныйСписокОткладывается(t *testing.T) {
	path := filepath.Join(t.TempDir(), "radio.json")
	os.WriteFile(path, []byte("{не json"), 0o600)
	r, err := Open(path, quiet(), "test")
	if err == nil || r == nil || len(r.Stations()) != 0 {
		t.Fatalf("ждали ошибку и пустой список: %v", err)
	}
	if _, err := os.Stat(path + ".broken"); err != nil {
		t.Error("испорченный файл должен быть отложен")
	}
}

type slowStation struct {
	mu    sync.Mutex
	title string
	next  chan struct{}
}

func (s *slowStation) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const metaint = 1000
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("icy-name", "Тест FM")
	w.Header().Set("icy-metaint", fmt.Sprint(metaint))
	audio := bytes.Repeat(silentFrame(), 3)[:metaint]
	for {
		s.mu.Lock()
		meta := []byte("StreamTitle='" + s.title + "';")
		s.mu.Unlock()
		n := (len(meta) + 15) / 16
		w.Write(audio)
		w.Write(append(append([]byte{byte(n)}, meta...), make([]byte, n*16-len(meta))...))
		w.(http.Flusher).Flush()
		select {
		case <-s.next:
		case <-r.Context().Done():
			return
		}
	}
}

func (s *slowStation) set(title string) {
	s.mu.Lock()
	s.title = title
	s.mu.Unlock()
	s.next <- struct{}{}
}

func waitEvent(t *testing.T, r *Radio) media.State {
	t.Helper()
	select {
	case st := <-r.Events():
		return st
	case <-time.After(3 * time.Second):
		t.Fatal("радио не сообщило о новом состоянии")
		return media.State{}
	}
}

func TestПотокДляОкнаИНазванияДляДвижка(t *testing.T) {
	st := &slowStation{title: "Кино - Кукушка", next: make(chan struct{})}
	station := httptest.NewServer(st)
	defer station.Close()

	r, _ := Open(filepath.Join(t.TempDir(), "radio.json"), quiet(), "test")
	s, _ := r.Add("", station.URL)
	if err := r.Play(s.ID); err != nil {
		t.Fatal(err)
	}

	window := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.ServeStream(w, req, req.URL.Query().Get("s"))
	}))
	defer window.Close()
	resp, err := http.Get(window.URL + "/?s=" + s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Header.Get("Content-Type") != "audio/mpeg" {
		t.Errorf("тип звука для окна: %q", resp.Header.Get("Content-Type"))
	}

	connected := waitEvent(t, r)
	if connected.Status != media.Playing || connected.Track.Title != "Тест FM" {
		t.Errorf("при подключении: %+v", connected)
	}
	if got := r.Stations()[0]; got.Name != "Тест FM" || got.AutoName {
		t.Errorf("станция без названия должна взять название из потока: %+v", got)
	}
	first := waitEvent(t, r)
	if first.Track.Artist() != "Кино" || first.Track.Title != "Кукушка" || !first.Track.Live || first.FromStart {
		t.Errorf("первая песня — начало пропущено: %+v", first)
	}
	st.set("Lana Del Rey - Summertime Sadness")
	second := waitEvent(t, r)
	if second.Track.Artist() != "Lana Del Rey" || !second.FromStart {
		t.Errorf("смена песни при прослушивании — слышна с начала: %+v", second)
	}

	buf := make([]byte, 4096)
	n, _ := io.ReadFull(resp.Body, buf[:1500])
	if n != 1500 || bytes.Contains(buf[:n], []byte("StreamTitle")) {
		t.Errorf("окно получило %d байт или вставки с названиями", n)
	}
	if got := r.Status(); !got.Connected || got.Artist != "Lana Del Rey" || got.Format != "MP3" {
		t.Errorf("состояние: %+v", got)
	}

	resp.Body.Close()
	stopped := waitEvent(t, r)
	if stopped.Status != media.Stopped {
		t.Errorf("после отключения окна: %+v", stopped)
	}
	r.Stop()
	if gone := waitEvent(t, r); !gone.Gone {
		t.Errorf("после выключения: %+v", gone)
	}
}

func TestНедоступнаяСтанцияВыключает(t *testing.T) {
	r, _ := Open(filepath.Join(t.TempDir(), "radio.json"), quiet(), "test")
	s, _ := r.Add("Нет такой", "http://127.0.0.1:1/live")
	r.Play(s.ID)
	rec := httptest.NewRecorder()
	r.ServeStream(rec, httptest.NewRequest("GET", "/", nil), s.ID)
	if rec.Code != http.StatusBadGateway {
		t.Errorf("код ответа окну: %d", rec.Code)
	}
	if got := r.Status(); got.Playing || !strings.Contains(got.Error, "не удалось подключиться") {
		t.Errorf("состояние: %+v", got)
	}
	rec = httptest.NewRecorder()
	r.ServeStream(rec, httptest.NewRequest("GET", "/", nil), s.ID)
	if rec.Code != http.StatusConflict {
		t.Errorf("выключенное радио: %d", rec.Code)
	}
}

func TestРадиоВыключаетсяБезОкна(t *testing.T) {
	r, _ := Open(filepath.Join(t.TempDir(), "radio.json"), quiet(), "test")
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return now }
	s, _ := r.Add("Тест", "http://127.0.0.1:1/")
	r.Play(s.ID)
	now = now.Add(listenTimeout + time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()
	deadline := time.Now().Add(7 * time.Second)
	for r.Status().Playing && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	got := r.Status()
	cancel()
	<-done
	if got.Playing || got.Error == "" {
		t.Errorf("радио без окна должно выключиться с пояснением: %+v", got)
	}
}
