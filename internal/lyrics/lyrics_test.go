package lyrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

func TestРазборLRC(t *testing.T) {
	text := "[ar:Тестовый исполнитель]\n[offset:+500]\r\n" +
		"[00:12.50]вторая строка\n" +
		"[00:02.00][01:02.25]<00:02.10>первая <00:02.40>строка\n" +
		"[00:20]\n" +
		"[00:30:75]строка с двоеточием\n" +
		"просто строка без отметки\n"
	got := ParseLRC(text)
	want := []Line{
		{1.5, "первая строка"}, {12, "вторая строка"}, {19.5, ""}, {30.25, "строка с двоеточием"}, {61.75, "первая строка"},
	}
	if len(got) != len(want) {
		t.Fatalf("строк %d, ожидалось %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Text != want[i].Text || got[i].At < want[i].At-0.001 || got[i].At > want[i].At+0.001 {
			t.Errorf("строка %d: %+v, ожидалось %+v", i, got[i], want[i])
		}
	}
	l := FromText(text, FromFile)
	if len(l.Synced) != 5 || l.Plain != "первая строка\nвторая строка\n\nстрока с двоеточием\nпервая строка" {
		t.Errorf("текст из LRC: %q", l.Plain)
	}
}

func TestТекстБезОтметокОстаётсяОбычным(t *testing.T) {
	l := FromText("  Куплет один\r\nКуплет два\n\nПрипев  ", FromTags)
	if len(l.Synced) != 0 || l.Plain != "Куплет один\nКуплет два\n\nПрипев" || l.Source != FromTags {
		t.Errorf("обычный текст: %+v", l)
	}
	if !FromText(" \n ", FromTags).Empty() {
		t.Error("пустой текст должен быть пустым")
	}
}

func TestКодировкиФайлаLRC(t *testing.T) {
	want := "Ёлка «тест» — №1"
	cp := []byte{0xA8, 0xEB, 0xEA, 0xE0, ' ', 0xAB, 't', 'e', 's', 't', 0xBB, ' ', 0x97, ' ', 0xB9, '1'}
	if got := decode(cp); strings.Replace(got, "test", "тест", 1) != want {
		t.Errorf("windows-1251: %q", got)
	}
	utf16le := []byte{0xFF, 0xFE}
	for _, r := range "Ёж" {
		utf16le = append(utf16le, byte(r), byte(r>>8))
	}
	if got := decode(utf16le); got != "Ёж" {
		t.Errorf("utf-16: %q", got)
	}
	if got := decode(append([]byte{0xEF, 0xBB, 0xBF}, "Ёж"...)); got != "Ёж" {
		t.Errorf("utf-8 с BOM: %q", got)
	}
}

func lrclib(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Client{Endpoint: srv.URL, UserAgent: "test-agent/1.0"}
}

func TestLRCLIBТочноеСовпадение(t *testing.T) {
	c := lrclib(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/get" || r.Header.Get("User-Agent") != "test-agent/1.0" ||
			q.Get("artist_name") != "Тестовая группа" || q.Get("track_name") != "Песня" ||
			q.Get("album_name") != "Альбом" || q.Get("duration") != "181" {
			t.Errorf("неожиданный запрос %s %s", r.URL.Path, r.URL.RawQuery)
		}
		w.Write([]byte(`{"id":1,"trackName":"Песня","artistName":"Тестовая группа","albumName":"Альбом","duration":181,
			"instrumental":false,"plainLyrics":"раз\nдва","syncedLyrics":"[00:01.00] раз\n[00:03.00] два"}`))
	})
	l, err := c.Find(context.Background(), Query{Artist: "Тестовая группа", Title: "Песня", Album: "Альбом", Duration: 180.6})
	if err != nil || len(l.Synced) != 2 || l.Synced[1].Text != "два" || l.Plain != "раз\nдва" || l.Source != FromLRCLIB {
		t.Fatalf("получено %+v, %v", l, err)
	}
}

func TestLRCLIBПоискВыбираетПодходящий(t *testing.T) {
	var paths []string
	c := lrclib(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/api/get" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"code":404,"name":"TrackNotFound","message":"Failed to find specified track"}`))
			return
		}
		w.Write([]byte(`[
			{"trackName":"Песня","artistName":"Чужая группа","duration":200,"syncedLyrics":"[00:01.00] чужое\n[00:02.00] чужое"},
			{"trackName":"Песня","artistName":"Тестовая группа","duration":260,"syncedLyrics":"[00:01.00] долгая\n[00:02.00] версия"},
			{"trackName":"Песня","artistName":"Тестовая группа","duration":199,"plainLyrics":"без отметок"},
			{"trackName":"Песня (feat. Гость)","artistName":"тестовая группа","duration":201,"syncedLyrics":"[00:01.00] наша\n[00:02.00] строка"},
			{"trackName":"Другая песня","artistName":"Тестовая группа","duration":200,"syncedLyrics":"[00:01.00] не та\n[00:02.00] песня"}
		]`))
	})
	l, err := c.Find(context.Background(), Query{Artist: "Тестовая группа", Title: "Песня", Album: "Альбом", Duration: 200})
	if err != nil || len(l.Synced) != 2 || l.Synced[0].Text != "наша" {
		t.Fatalf("выбран не тот текст: %+v, %v", l, err)
	}
	if strings.Join(paths, ",") != "/api/get,/api/search" {
		t.Errorf("запросы: %v", paths)
	}
}

func TestLRCLIBИщетБезПриписокВНазвании(t *testing.T) {
	var tracks []string
	c := lrclib(t, func(w http.ResponseWriter, r *http.Request) {
		tr := r.URL.Query().Get("track_name")
		tracks = append(tracks, tr)
		if tr != "Песня" {
			w.Write([]byte(`[]`))
			return
		}
		w.Write([]byte(`[{"trackName":"Песня","artistName":"Тестовая группа","duration":0,"plainLyrics":"найдено"}]`))
	})
	l, err := c.Find(context.Background(), Query{Artist: "Тестовая группа", Title: "Песня (feat. Гость) - Remastered 2011"})
	if err != nil || l.Plain != "найдено" {
		t.Fatalf("получено %+v, %v", l, err)
	}
	if len(tracks) != 2 || tracks[1] != "Песня" {
		t.Errorf("названия в запросах: %q", tracks)
	}
	for in, want := range map[string]string{
		"Песня (feat. Гость)":           "Песня",
		"Песня [ft. Гость, Ещё]":        "Песня",
		"Песня feat. Гость":             "Песня",
		"Песня - 2011 Remaster":         "Песня",
		"Песня (Remastered 2009)":       "Песня",
		"Песня (Remix)":                 "Песня (Remix)",
		"Песня (remix) [feat. A, B, C]": "Песня (remix)",
	} {
		if got := CleanTitle(in); got != want {
			t.Errorf("CleanTitle(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}

func TestLRCLIBИщетОднойСтрокой(t *testing.T) {
	var asked []string
	c := lrclib(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		asked = append(asked, q.Get("q"))
		if q.Get("q") != "Тестовая группа Песня" {
			w.Write([]byte(`[]`))
			return
		}
		w.Write([]byte(`[{"trackName":"песня","artistName":"Тестовая Группа","duration":181,"plainLyrics":"найдено общим поиском"}]`))
	})
	l, err := c.Find(context.Background(), Query{Artist: "Тестовая группа", Title: "Песня (feat. Гость)", Duration: 180})
	if err != nil || l.Plain != "найдено общим поиском" {
		t.Fatalf("общий поиск: %+v, %v", l, err)
	}
	if len(asked) != 3 || asked[0] != "" || asked[1] != "" {
		t.Errorf("общий поиск идёт после поиска по полям: %q", asked)
	}
}

func TestLRCLIBНетТекстаИСбой(t *testing.T) {
	c := lrclib(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`[]`)) })
	if _, err := c.Find(context.Background(), Query{Artist: "Тестовая группа", Title: "Песня"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("пустой поиск: %v", err)
	}
	c = lrclib(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) })
	if _, err := c.Find(context.Background(), Query{Artist: "Тестовая группа", Title: "Песня"}); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("сбой сервера не должен выглядеть как «не найдено»: %v", err)
	}
	c = lrclib(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"trackName":"Песня","artistName":"Тестовая группа","instrumental":true}]`))
	})
	if l, err := c.Find(context.Background(), Query{Artist: "Тестовая группа", Title: "Песня"}); err != nil || !l.Instrumental {
		t.Errorf("песня без слов: %+v, %v", l, err)
	}
}

func track(title, url string) media.Track {
	return media.Track{Title: title, Artists: []string{"Тестовая группа"}, Album: "Альбом", Length: 3 * time.Minute, URL: url}
}

func TestХранилищеБерётФайлРядомСПесней(t *testing.T) {
	dir := t.TempDir()
	song := filepath.Join(dir, "01 Песня.mp3")
	os.WriteFile(song, []byte("не mp3"), 0o644)
	os.WriteFile(filepath.Join(dir, "01 Песня.lrc"), []byte("[00:01.00]из файла\n[00:05.00]рядом"), 0o644)
	var calls atomic.Int32
	s := New(t.TempDir(), func(context.Context, Query) (Lyrics, error) {
		calls.Add(1)
		return Lyrics{Plain: "из интернета", Source: FromLRCLIB}, nil
	}, nil)
	tr := track("Песня", "file://"+filepath.ToSlash(song))
	if r := s.Get(tr); r.State != Loading {
		t.Fatalf("сначала ищем: %+v", r)
	}
	s.Wait()
	r := s.Get(tr)
	if r.State != Found || r.Source != FromFile || len(r.Synced) != 2 || calls.Load() != 0 {
		t.Errorf("текст из .lrc рядом с песней: %+v, запросов %d", r, calls.Load())
	}
}

func id3WithLyrics(text string) []byte {
	body := append([]byte{3}, "rus"...)
	body = append(body, 0)
	body = append(body, text...)
	syncsafe := func(n int) []byte {
		return []byte{byte(n >> 21 & 0x7f), byte(n >> 14 & 0x7f), byte(n >> 7 & 0x7f), byte(n & 0x7f)}
	}
	frame := append([]byte("USLT"), syncsafe(len(body))...)
	frame = append(frame, 0, 0)
	frame = append(frame, body...)
	out := append([]byte("ID3"), 4, 0, 0)
	out = append(out, syncsafe(len(frame))...)
	return append(out, frame...)
}

func TestТекстИзТеговФайла(t *testing.T) {
	song := filepath.Join(t.TempDir(), "песня.mp3")
	os.WriteFile(song, id3WithLyrics("Строка из тегов\nЕщё строка"), 0o644)
	l := fromTags(song)
	if l.Plain != "Строка из тегов\nЕщё строка" || l.Source != FromTags {
		t.Errorf("текст из тегов: %+v", l)
	}
	if !fromTags(filepath.Join(t.TempDir(), "нет.mp3")).Empty() {
		t.Error("у отсутствующего файла текста нет")
	}
}

func TestХранилищеКэшируетИПовторяет(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	answer := func(context.Context, Query) (Lyrics, error) {
		calls.Add(1)
		return Lyrics{Plain: "сохранённый текст", Source: FromLRCLIB}, nil
	}
	s := New(dir, func(ctx context.Context, q Query) (Lyrics, error) { return answer(ctx, q) }, nil)
	s.Now = func() time.Time { return now }
	tr := track("Песня", "")
	s.Get(tr)
	s.Wait()
	for i := 0; i < 3; i++ {
		if r := s.Get(tr); r.State != Found || r.Plain != "сохранённый текст" {
			t.Fatalf("найденный текст: %+v", r)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("LRCLIB спрашивается один раз: %d", calls.Load())
	}

	again := New(dir, func(context.Context, Query) (Lyrics, error) {
		calls.Add(1)
		return Lyrics{}, errors.New("сеть недоступна")
	}, nil)
	again.Get(tr)
	again.Wait()
	if r := again.Get(tr); r.State != Found || r.Plain != "сохранённый текст" || calls.Load() != 1 {
		t.Errorf("после перезапуска текст берётся с диска: %+v, запросов %d", r, calls.Load())
	}

	answer = func(context.Context, Query) (Lyrics, error) {
		calls.Add(1)
		return Lyrics{}, ErrNotFound
	}
	missing := track("Песня без текста", "")
	s.Get(missing)
	s.Wait()
	if r := s.Get(missing); r.State != Missing {
		t.Errorf("не найден: %+v", r)
	}
	before := calls.Load()
	now = now.Add(retryMissing - time.Minute)
	s.Get(missing)
	s.Wait()
	if calls.Load() != before {
		t.Error("ненайденный текст не ищется снова раньше срока")
	}
	now = now.Add(2 * time.Minute)
	s.Get(missing)
	s.Wait()
	if calls.Load() != before+1 {
		t.Error("ненайденный текст ищется снова через полсуток")
	}

	answer = func(context.Context, Query) (Lyrics, error) {
		calls.Add(1)
		return Lyrics{}, errors.New("сеть недоступна")
	}
	broken := track("Песня при сбое", "")
	s.Get(broken)
	s.Wait()
	if r := s.Get(broken); r.State != Failed {
		t.Errorf("сбой: %+v", r)
	}
	before = calls.Load()
	now = now.Add(retryFailed + time.Second)
	s.Get(broken)
	s.Wait()
	if calls.Load() != before+1 {
		t.Error("после сбоя текст ищется снова через пару минут")
	}
}
