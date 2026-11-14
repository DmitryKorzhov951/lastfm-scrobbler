package lyrics

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
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

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func songIn(t *testing.T, dir string) string {
	t.Helper()
	song := filepath.Join(dir, "07 Песня.flac")
	write(t, song, "не flac")
	return song
}

func TestФайлыСИменемПесни(t *testing.T) {
	cases := []struct {
		name   string
		artist string
		title  string
		file   string
		text   string
	}{
		{"txt рядом с песней", "Тестовая группа", "Песня", "07 Песня.TXT", "простой текст"},
		{"исполнитель и название", "Тестовая группа", "Песня", "Тестовая группа - Песня.txt", "по имени"},
		{"подпапка lyrics", "Тестовая группа", "Песня", "LYRICS/тестовая группа - песня.lrc", "[00:01.00]из подпапки\n[00:02.00]вторая"},
		{"запрещённые знаки заменены", "Группа: тест", "Что? Где", "Группа_ тест - Что_ Где.lrc", "с подчёркиваниями"},
		{"запрещённые знаки убраны", "Группа: тест", "Что? Где", "Группа тест - Что Где.txt", "без знаков"},
		{"без гостей в названии", "Тестовая группа", "Песня (feat. Гость)", "Тестовая группа - Песня.txt", "без feat"},
		{"главный исполнитель", "Тестовая группа, Гость", "Песня", "Тестовая группа - Песня.txt", "первый из двух"},
		{"лишние пробелы", "Тестовая группа", "Песня", "Тестовая  группа -  Песня.txt", "пробелы"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			song := songIn(t, dir)
			write(t, filepath.Join(dir, filepath.FromSlash(c.file)), c.text)
			l := nearSong(song, names(c.artist, c.title))
			if l.Empty() || l.Source != FromFile || !strings.Contains(l.Plain, strings.Split(strings.TrimPrefix(c.text, "[00:01.00]"), "\n")[0]) {
				t.Errorf("файл %q не найден для «%s — %s»: %+v", c.file, c.artist, c.title, l)
			}
		})
	}
}

func TestФайлРядомСПесниейВажнееИменного(t *testing.T) {
	dir := t.TempDir()
	song := songIn(t, dir)
	write(t, filepath.Join(dir, "Тестовая группа - Песня.lrc"), "[00:01.00]по имени\n[00:02.00]строка")
	write(t, filepath.Join(dir, "07 Песня.lrc"), "[00:01.00]рядом\n[00:02.00]строка")
	write(t, filepath.Join(dir, "Тестовая группа - Другая.lrc"), "[00:01.00]чужой\n[00:02.00]файл")
	if l := nearSong(song, names("Тестовая группа", "Песня")); len(l.Synced) != 2 || l.Synced[0].Text != "рядом" {
		t.Errorf("файл с именем песни главнее: %+v", l)
	}
	os.Remove(filepath.Join(dir, "07 Песня.lrc"))
	if l := nearSong(song, names("Тестовая группа", "Третья")); !l.Empty() {
		t.Errorf("чужие файлы не подходят: %+v", l)
	}
}

func TestПапкаСТекстамиДляЛюбогоПлеера(t *testing.T) {
	folder := t.TempDir()
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	s := New(t.TempDir(), func(context.Context, Query) (Lyrics, error) {
		calls.Add(1)
		return Lyrics{}, ErrNotFound
	}, nil)
	s.Now = func() time.Time { return now }
	s.Folder = func() string { return folder }
	tr := media.Track{Title: "Песня", Artists: []string{"Тестовая группа"}, Length: 3 * time.Minute}
	s.Get(tr)
	s.Wait()
	if r := s.Get(tr); r.State != Missing || calls.Load() != 1 {
		t.Fatalf("сначала текста нет: %+v, запросов %d", r, calls.Load())
	}
	write(t, filepath.Join(folder, "Тестовая группа - Песня.lrc"), "[00:01.00]положили\n[00:04.00]в папку")
	if r := s.Get(tr); r.State != Missing {
		t.Errorf("папка перечитывается не чаще раза в %v: %+v", recheckGap, r)
	}
	now = now.Add(recheckGap)
	r := s.Get(tr)
	if r.State != Found || r.Source != FromFolder || len(r.Synced) != 2 || r.Synced[1].Text != "в папку" {
		t.Fatalf("файл из папки с текстами подхватывается без перезапуска: %+v", r)
	}
	if r := s.Get(tr); r.State != Found || calls.Load() != 1 {
		t.Errorf("сайты больше не спрашиваются: %+v, запросов %d", r, calls.Load())
	}

	other := media.Track{Title: "Вторая", Artists: []string{"Тестовая группа"}, Length: 3 * time.Minute}
	write(t, filepath.Join(folder, "Тестовая группа - Вторая.txt"), "сразу в папке")
	s.Get(other)
	s.Wait()
	if r := s.Get(other); r.State != Found || r.Source != FromFolder || calls.Load() != 1 {
		t.Errorf("папка проверяется до сайтов: %+v, запросов %d", r, calls.Load())
	}
}

func TestПапкаВажнееТеговАРядомВажнееПапки(t *testing.T) {
	dir, folder := t.TempDir(), t.TempDir()
	song := filepath.Join(dir, "песня.mp3")
	write(t, song, string(id3WithLyrics("из тегов")))
	s := New("", nil, nil)
	s.Folder = func() string { return folder }
	tr := track("Песня", "file://"+filepath.ToSlash(song))
	if l := s.local(tr); l.Source != FromTags {
		t.Errorf("только теги: %+v", l)
	}
	write(t, filepath.Join(folder, "Тестовая группа - Песня.txt"), "из папки")
	if l := s.local(tr); l.Source != FromFolder {
		t.Errorf("папка с текстами важнее тегов: %+v", l)
	}
	write(t, filepath.Join(dir, "песня.txt"), "рядом")
	if l := s.local(tr); l.Source != FromFile || l.Plain != "рядом" {
		t.Errorf("файл рядом с песней важнее всего: %+v", l)
	}
}

func flacWithComments(comments ...string) []byte {
	le := func(n int) []byte { return binary.LittleEndian.AppendUint32(nil, uint32(n)) }
	body := append(le(4), "test"...)
	body = append(body, le(len(comments))...)
	for _, c := range comments {
		body = append(body, le(len(c))...)
		body = append(body, c...)
	}
	out := append([]byte("fLaC"), 0x80|4, byte(len(body)>>16), byte(len(body)>>8), byte(len(body)))
	return append(out, body...)
}

func id3WithTXXX(desc, text string) []byte {
	body := append([]byte{3}, desc...)
	body = append(body, 0)
	body = append(body, text...)
	syncsafe := func(n int) []byte {
		return []byte{byte(n >> 21 & 0x7f), byte(n >> 14 & 0x7f), byte(n >> 7 & 0x7f), byte(n & 0x7f)}
	}
	frame := append([]byte("TXXX"), syncsafe(len(body))...)
	frame = append(frame, 0, 0)
	frame = append(frame, body...)
	out := append([]byte("ID3"), 4, 0, 0)
	out = append(out, syncsafe(len(frame))...)
	return append(out, frame...)
}

func TestТекстВДругихПоляхТегов(t *testing.T) {
	dir := t.TempDir()
	flac := filepath.Join(dir, "песня.flac")
	write(t, flac, string(flacWithComments("TITLE=Песня", "UNSYNCEDLYRICS=Строка из FLAC\nвторая")))
	if l := fromTags(flac); l.Plain != "Строка из FLAC\nвторая" || l.Source != FromTags {
		t.Errorf("UNSYNCEDLYRICS во FLAC: %+v", l)
	}
	both := filepath.Join(dir, "оба.flac")
	write(t, both, string(flacWithComments("UNSYNCEDLYRICS=запасной", "LYRICS=[00:01.00]главный\n[00:02.00]текст")))
	if l := fromTags(both); len(l.Synced) != 2 || l.Synced[0].Text != "главный" {
		t.Errorf("LYRICS главнее UNSYNCEDLYRICS: %+v", l)
	}
	mp3 := filepath.Join(dir, "песня.mp3")
	write(t, mp3, string(id3WithTXXX("LYRICS", "Строка из TXXX")))
	if l := fromTags(mp3); l.Plain != "Строка из TXXX" {
		t.Errorf("TXXX:LYRICS в mp3: %+v", l)
	}
	other := filepath.Join(dir, "другое.mp3")
	write(t, other, string(id3WithTXXX("MOOD", "весёлое")))
	if l := fromTags(other); !l.Empty() {
		t.Errorf("чужие поля TXXX не текст песни: %+v", l)
	}
}

func ovh(t *testing.T, h http.HandlerFunc) *OVH {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &OVH{Endpoint: srv.URL, UserAgent: "test-agent/1.0"}
}

func TestLyricsOVH(t *testing.T) {
	var uri string
	c := ovh(t, func(w http.ResponseWriter, r *http.Request) {
		uri = r.RequestURI
		if r.Header.Get("User-Agent") != "test-agent/1.0" {
			t.Errorf("User-Agent: %q", r.Header.Get("User-Agent"))
		}
		w.Write([]byte(`{"lyrics":"Paroles de la chanson Песня par Группа/Дуэт\r\nСтрока раз\nСтрока два\n\n\n\nСтрока три"}`))
	})
	l, err := c.Find(context.Background(), Query{Artist: "Группа/Дуэт", Title: "Песня? (feat. Гость)"})
	if err != nil || l.Plain != "Строка раз\nСтрока два\n\nСтрока три" || l.Source != FromOVH || len(l.Synced) != 0 {
		t.Fatalf("текст с lyrics.ovh: %+v, %v", l, err)
	}
	if !strings.HasPrefix(uri, "/v1/") || !strings.Contains(uri, "%2F") || !strings.Contains(uri, "%3F") || strings.Contains(uri, "feat") {
		t.Errorf("адрес запроса: %s", uri)
	}
}

func TestLyricsOVHЧерезСтроку(t *testing.T) {
	c := ovh(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"lyrics":"Первая\n\nВторая\n\nТретья\n\nЧетвёртая\n\n\n\nПятая\n\nШестая"}`))
	})
	l, err := c.Find(context.Background(), Query{Artist: "Группа", Title: "Песня"})
	if err != nil || l.Plain != "Первая\nВторая\nТретья\nЧетвёртая\n\nПятая\nШестая" {
		t.Errorf("строки через одну пустую склеиваются: %q, %v", l.Plain, err)
	}
}

func TestLyricsOVHНетТекстаИСбой(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{
		"404": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"No lyrics found"}`))
		},
		"пустой": func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"lyrics":"  "}`)) },
	} {
		if _, err := ovh(t, h).Find(context.Background(), Query{Artist: "Группа", Title: "Песня"}); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	c := ovh(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	if _, err := c.Find(context.Background(), Query{Artist: "Группа", Title: "Песня"}); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("сбой lyrics.ovh не «не найдено»: %v", err)
	}
}

type probe struct {
	name  string
	calls *[]string
	find  func(q Query) (Lyrics, error)
}

func (p probe) Find(_ context.Context, q Query) (Lyrics, error) {
	*p.calls = append(*p.calls, p.name+":"+q.Artist+"/"+q.Title)
	return p.find(q)
}

func TestЦепочкаИсточников(t *testing.T) {
	var calls []string
	missing := func(Query) (Lyrics, error) { return Lyrics{}, ErrNotFound }
	a := probe{"lrclib", &calls, missing}
	b := probe{"ovh", &calls, func(q Query) (Lyrics, error) {
		if q.Artist == "Правильная группа" {
			return Lyrics{Plain: "нашёлся", Source: FromOVH}, nil
		}
		return Lyrics{}, ErrNotFound
	}}
	fixes := 0
	c := &Chain{Sources: []Finder{a.Find, b.Find}, Fix: func(_ context.Context, artist, title string) (string, string, error) {
		fixes++
		if artist != "Тестовая группа, Гость" || title != "Песня" {
			t.Errorf("исправляется исходное название: %s — %s", artist, title)
		}
		return "Правильная группа", "Песня", nil
	}}
	l, err := c.Find(context.Background(), Query{Artist: "Тестовая группа, Гость", Title: "Песня"})
	if err != nil || l.Plain != "нашёлся" {
		t.Fatalf("найдено по исправленному имени: %+v, %v", l, err)
	}
	want := "lrclib:Тестовая группа, Гость/Песня lrclib:Тестовая группа/Песня lrclib:Правильная группа/Песня " +
		"ovh:Тестовая группа, Гость/Песня ovh:Тестовая группа/Песня ovh:Правильная группа/Песня"
	if strings.Join(calls, " ") != want || fixes != 1 {
		t.Errorf("порядок запросов:\n%s\nожидался\n%s\nисправлений %d", strings.Join(calls, " "), want, fixes)
	}

	calls = nil
	c.Fix = func(context.Context, string, string) (string, string, error) {
		return "тестовая ГРУППА", "песня", nil
	}
	if _, err := c.Find(context.Background(), Query{Artist: "Тестовая группа", Title: "Песня"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("ничего не нашлось: %v", err)
	}
	if len(calls) != 2 {
		t.Errorf("то же имя в другом регистре не спрашивается снова: %v", calls)
	}
}

func TestЦепочкаЧестноСообщаетОСбоях(t *testing.T) {
	var calls []string
	down := errors.New("сайт недоступен")
	broken := probe{"lrclib", &calls, func(Query) (Lyrics, error) { return Lyrics{}, down }}
	empty := probe{"ovh", &calls, func(Query) (Lyrics, error) { return Lyrics{}, ErrNotFound }}
	q := Query{Artist: "Тестовая группа & Гость", Title: "Песня"}

	c := &Chain{Sources: []Finder{broken.Find, empty.Find}}
	_, err := c.Find(context.Background(), q)
	var part *Incomplete
	if !errors.As(err, &part) || !errors.Is(err, down) || errors.Is(err, ErrNotFound) {
		t.Errorf("один сайт упал, другой ответил «нет»: %v", err)
	}
	if strings.Join(calls, " ") != "lrclib:Тестовая группа & Гость/Песня ovh:Тестовая группа & Гость/Песня ovh:Тестовая группа/Песня" {
		t.Errorf("упавший сайт не мучается остальными вариантами: %v", calls)
	}

	c = &Chain{Sources: []Finder{broken.Find, broken.Find}}
	if _, err := c.Find(context.Background(), q); !errors.Is(err, down) || errors.As(err, &part) {
		t.Errorf("все сайты недоступны — это сбой: %v", err)
	}

	c = &Chain{Sources: []Finder{empty.Find}, Fix: func(context.Context, string, string) (string, string, error) {
		return "", "", fmt.Errorf("Last.fm недоступен")
	}}
	if _, err := c.Find(context.Background(), q); !errors.Is(err, ErrNotFound) {
		t.Errorf("сбой исправления названий не мешает: %v", err)
	}
}

func TestХранилищеПослеЧастичногоСбояИщетСкорее(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	s := New("", func(context.Context, Query) (Lyrics, error) {
		calls.Add(1)
		return Lyrics{}, &Incomplete{Err: errors.New("lyrics.ovh: ответ 503")}
	}, nil)
	s.Now = func() time.Time { return now }
	tr := track("Песня", "")
	s.Get(tr)
	s.Wait()
	if r := s.Get(tr); r.State != Missing {
		t.Fatalf("часть сайтов ответила «нет» — текст не найден: %+v", r)
	}
	now = now.Add(retryIncomplete - time.Minute)
	s.Get(tr)
	s.Wait()
	if calls.Load() != 1 {
		t.Error("раньше срока не ищется")
	}
	now = now.Add(2 * time.Minute)
	s.Get(tr)
	s.Wait()
	if calls.Load() != 2 {
		t.Errorf("через %v ищется снова, а не через полсуток", retryIncomplete)
	}
}

func TestГлавныйИсполнитель(t *testing.T) {
	for in, want := range map[string]string{
		"Тестовая группа":              "Тестовая группа",
		"Группа, Гость":                "Группа",
		"Группа & Гость":               "Группа",
		"Группа feat. Гость":           "Группа",
		"Группа x Гость":               "Группа",
		"Группа и Гость":               "Группа",
		"+Группа":                      "+Группа",
		"Charli XCX":                   "Charli XCX",
		"  Группа ft. Гость, Ещё один": "Группа",
	} {
		if got := mainArtist(in); got != want {
			t.Errorf("mainArtist(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}

func TestИменаФайловБезПустыхЧастей(t *testing.T) {
	for _, n := range names("???", "Песня") {
		if strings.HasPrefix(n, " ") || strings.HasPrefix(n, "-") {
			t.Errorf("имя без исполнителя: %q", n)
		}
	}
	if got := names("Группа", "Песня"); len(got) != 1 || got[0] != "Группа - Песня" {
		t.Errorf("простые имена без повторов: %q", got)
	}
}
