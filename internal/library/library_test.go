package library

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"
)

var msk = time.FixedZone("MSK", 3*3600)

func at(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, msk) }

func play(t time.Time, artist, title, album string) Play {
	return Play{At: t, Artist: artist, Title: title, Album: album}
}

func openTemp(t *testing.T) (*Store, string, string) {
	t.Helper()
	dir := t.TempDir()
	path, meta := filepath.Join(dir, "library.jsonl"), filepath.Join(dir, "library.json")
	s, err := Open(path, meta)
	if err != nil {
		t.Fatal(err)
	}
	return s, path, meta
}

func TestПовторыНеДобавляютсяИИсторияПереживаетПерезапуск(t *testing.T) {
	s, path, meta := openTemp(t)
	plays := []Play{
		play(at(2026, 11, 5, 18, 0), "Кино", "Группа крови", "Группа крови"),
		play(at(2026, 11, 5, 18, 5), "Кино", "Кукушка", ""),
	}
	if n, err := s.Add(plays); err != nil || n != 2 {
		t.Fatalf("добавлено %d, %v", n, err)
	}
	if n, _ := s.Add([]Play{play(at(2026, 11, 5, 18, 0), "КИНО", "группа  крови", "x")}); n != 0 {
		t.Errorf("повтор с другим регистром добавлен: %d", n)
	}
	if n, _ := s.Add([]Play{{Artist: "Кино", Title: "без времени"}, play(at(2026, 1, 1, 0, 0), "", "x", "")}); n != 0 {
		t.Errorf("добавлены пустые прослушивания: %d", n)
	}
	if err := s.SaveMeta(Meta{User: "korzhov", Complete: true}); err != nil {
		t.Fatal(err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`[1793890000,"Кин`)
	f.Close()

	s2, err := Open(path, meta)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Len() != 2 {
		t.Errorf("после перезапуска %d прослушиваний", s2.Len())
	}
	if m := s2.Meta(); m.User != "korzhov" || !m.Complete {
		t.Errorf("состояние загрузки: %+v", m)
	}
	data, _ := os.ReadFile(path)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Errorf("оборванная строка не убрана: %q", data)
	}
}

func TestНеделяНачинаетсяВЧетверг(t *testing.T) {
	cases := []struct {
		now  time.Time
		from time.Time
	}{
		{at(2026, 11, 5, 0, 0), at(2026, 11, 5, 0, 0)},
		{at(2026, 11, 4, 23, 59), at(2026, 10, 29, 0, 0)},
		{at(2026, 11, 8, 12, 0), at(2026, 11, 5, 0, 0)},
		{at(2026, 11, 11, 23, 0), at(2026, 11, 5, 0, 0)},
		{at(2026, 11, 12, 0, 0), at(2026, 11, 12, 0, 0)},
	}
	for _, c := range cases {
		from, to, err := Bounds("week", 0, c.now)
		if err != nil {
			t.Fatal(err)
		}
		if !from.Equal(c.from) || !to.Equal(c.from.AddDate(0, 0, 7)) {
			t.Errorf("%s: неделя %s – %s", c.now, from, to)
		}
	}
	from, _, _ := Bounds("week", -1, at(2026, 11, 6, 10, 0))
	if !from.Equal(at(2026, 10, 29, 0, 0)) {
		t.Errorf("прошлая неделя начинается %s", from)
	}
	from, to, _ := Bounds("month", -11, at(2026, 11, 6, 10, 0))
	if !from.Equal(at(2025, 12, 1, 0, 0)) || !to.Equal(at(2026, 1, 1, 0, 0)) {
		t.Errorf("месяц: %s – %s", from, to)
	}
	from, to, _ = Bounds("year", -1, at(2026, 11, 6, 10, 0))
	if !from.Equal(at(2025, 1, 1, 0, 0)) || !to.Equal(at(2026, 1, 1, 0, 0)) {
		t.Errorf("год: %s – %s", from, to)
	}
}

func TestПодписиПериодов(t *testing.T) {
	cases := map[string]string{}
	for _, c := range []struct {
		kind string
		now  time.Time
		want string
	}{
		{"week", at(2026, 11, 6, 10, 0), "5–11 ноября 2026"},
		{"week", at(2026, 10, 30, 10, 0), "29 октября – 4 ноября 2026"},
		{"week", at(2027, 1, 2, 10, 0), "31 декабря 2026 – 6 января 2027"},
		{"month", at(2026, 11, 6, 10, 0), "ноябрь 2026"},
		{"year", at(2026, 11, 6, 10, 0), "2026 год"},
	} {
		from, to, _ := Bounds(c.kind, 0, c.now)
		if got := label(c.kind, from, to); got != c.want {
			cases[c.want] = got
		}
	}
	for want, got := range cases {
		t.Errorf("подпись %q, ожидалась %q", got, want)
	}
}

func TestТопНеделиСЧетвергаИЛишниеПрослушивания(t *testing.T) {
	s, _, _ := openTemp(t)
	s.Add([]Play{
		play(at(2026, 11, 4, 22, 0), "Земфира", "Хочешь?", "Прости меня моя любовь"),
		play(at(2026, 11, 4, 23, 0), "Земфира", "Хочешь?", "Прости меня моя любовь"),
		play(at(2026, 11, 5, 9, 0), "Кино", "Группа крови", "Группа крови"),
		play(at(2026, 11, 5, 10, 0), "Кино", "Кукушка", "Последний герой"),
		play(at(2026, 11, 6, 9, 0), "Сплин", "Выхода нет", "Гранатовый альбом"),
	})
	now := at(2026, 11, 6, 12, 0)
	local := []Play{
		play(at(2026, 11, 6, 11, 0), "Кино", "Группа крови", "Группа крови"),
		play(at(2026, 11, 6, 9, 0), "Сплин", "Выхода нет", "Гранатовый альбом"),
	}
	top, err := s.Top("artists", "week", 0, now, local)
	if err != nil {
		t.Fatal(err)
	}
	if top.Total != 4 || len(top.Items) != 2 {
		t.Fatalf("неделя: всего %d, строк %d: %+v", top.Total, len(top.Items), top.Items)
	}
	if top.Items[0].Artist != "Кино" || top.Items[0].Count != 3 || top.Items[0].Rank != 1 {
		t.Errorf("первое место: %+v", top.Items[0])
	}
	if !top.Period.Live || top.Period.HasNext || !top.Period.HasPrev || top.Period.Label != "5–11 ноября 2026" {
		t.Errorf("период: %+v", top.Period)
	}

	tracks, _ := s.Top("tracks", "week", -1, now, local)
	if len(tracks.Items) != 1 || tracks.Items[0].Title != "Хочешь?" || tracks.Items[0].Count != 2 {
		t.Errorf("прошлая неделя: %+v", tracks.Items)
	}
	if tracks.Period.HasPrev || !tracks.Period.HasNext || tracks.Period.Live {
		t.Errorf("прошлая неделя, период: %+v", tracks.Period)
	}

	albums, _ := s.Top("albums", "month", 0, now, nil)
	if albums.Total != 5 || albums.Items[0].Album != "Прости меня моя любовь" || albums.Items[0].Artist != "Земфира" {
		t.Errorf("альбомы месяца: %+v", albums)
	}
	if _, err := s.Top("songs", "week", 0, now, nil); err == nil {
		t.Error("неизвестный вид топа принят")
	}
}

func TestТопДесятьСОдинаковымСчётомПоАлфавиту(t *testing.T) {
	s := mustMemory(t)
	base := at(2026, 3, 1, 0, 0)
	var plays []Play
	names := []string{"Б", "А", "Г", "В", "Е", "Д", "З", "Ж", "К", "И", "М", "Л"}
	for i, n := range names {
		plays = append(plays, play(base.Add(time.Duration(i)*time.Hour), n, "трек", ""))
	}
	plays = append(plays, play(base.Add(100*time.Hour), "Л", "трек 2", ""))
	s.Add(plays)
	top, _ := s.Top("artists", "year", 0, at(2026, 6, 1, 0, 0), nil)
	got := []string{}
	for _, it := range top.Items {
		got = append(got, it.Artist)
	}
	want := []string{"Л", "А", "Б", "В", "Г", "Д", "Е", "Ж", "З", "И"}
	if len(got) != TopSize || !equal(got, want) {
		t.Errorf("топ: %v", got)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mustMemory(t *testing.T) *Store {
	t.Helper()
	s, err := Open("", "")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestКарточкиТрекаАльбомаИсполнителя(t *testing.T) {
	s := mustMemory(t)
	s.Add([]Play{
		play(at(2024, 3, 12, 20, 0), "Кино", "Группа крови", "Группа крови"),
		play(at(2026, 10, 1, 20, 0), "Кино", "Группа крови", "Группа крови"),
		play(at(2026, 11, 5, 20, 0), "Кино", "Группа крови", "Группа крови"),
		play(at(2026, 11, 5, 21, 0), "кино", "Война", "Группа крови"),
		play(at(2026, 11, 6, 8, 0), "Кино", "Кукушка", "Последний герой"),
	})
	now := at(2026, 11, 6, 12, 0)
	local := []Play{play(at(2026, 11, 6, 11, 0), "Кино", "Группа Крови", "Группа крови")}

	tr, err := s.Card("track", "КИНО", "группа крови", "", now, local)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Count != 4 || tr.Week != 2 || tr.Month != 2 || tr.Year != 3 {
		t.Errorf("трек: %+v", tr)
	}
	if tr.Title != "Группа Крови" || tr.Album != "Группа крови" || tr.AlbumCount != 5 || tr.ArtistCount != 6 {
		t.Errorf("трек, название и соседи: %q %q %d %d", tr.Title, tr.Album, tr.AlbumCount, tr.ArtistCount)
	}
	if !tr.First.Equal(at(2024, 3, 12, 20, 0)) || !tr.Last.Equal(at(2026, 11, 6, 11, 0)) {
		t.Errorf("первый и последний раз: %s, %s", tr.First, tr.Last)
	}

	al, _ := s.Card("album", "Кино", "", "группа крови", now, local)
	if al.Count != 5 || len(al.Tracks) != 2 || al.Tracks[0].Title != "Группа Крови" || al.Tracks[0].Count != 4 {
		t.Errorf("альбом: %+v", al)
	}

	ar, _ := s.Card("artist", "кино", "", "", now, local)
	if ar.Count != 6 || ar.Artist != "Кино" || len(ar.Tracks) != 3 || len(ar.Albums) != 2 || ar.Albums[0].Count != 5 {
		t.Errorf("исполнитель: %+v", ar)
	}

	none, _ := s.Card("track", "Кино", "Звезда по имени Солнце", "", now, local)
	if none.Count != 0 || none.First != nil {
		t.Errorf("непрослушанный трек: %+v", none)
	}
	if _, err := s.Card("genre", "Кино", "", "", now, nil); err == nil {
		t.Error("неизвестный вид карточки принят")
	}
}

type fakeSource struct {
	mu      sync.Mutex
	plays   []Play
	perPage int
	calls   []string
	failAt  int
	failed  bool
	user    string
}

func (f *fakeSource) RecentTracks(ctx context.Context, user string, page int, from, to time.Time) (Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.user = user
	f.calls = append(f.calls, from.Format(time.RFC3339)+"|"+to.Format(time.RFC3339))
	if f.failAt > 0 && len(f.calls) == f.failAt && !f.failed {
		f.failed = true
		return Page{}, errors.New("нет связи")
	}
	var sel []Play
	for _, p := range f.plays {
		if (!from.IsZero() && p.At.Before(from)) || (!to.IsZero() && !p.At.Before(to)) {
			continue
		}
		sel = append(sel, p)
	}
	sort.Slice(sel, func(i, j int) bool { return sel[i].At.After(sel[j].At) })
	total := len(sel)
	pages := (total + f.perPage - 1) / f.perPage
	lo := (page - 1) * f.perPage
	if lo > total {
		lo = total
	}
	hi := lo + f.perPage
	if hi > total {
		hi = total
	}
	return Page{Plays: append([]Play(nil), sel[lo:hi]...), TotalPages: pages, Total: total}, nil
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestЗагрузкаВсейИсторииСПродолжениемПослеОбрыва(t *testing.T) {
	s, path, meta := openTemp(t)
	src := &fakeSource{perPage: 3, failAt: 3}
	start := at(2026, 1, 1, 0, 0)
	for i := 0; i < 10; i++ {
		src.plays = append(src.plays, play(start.Add(time.Duration(i)*time.Hour), "Кино", "трек", ""))
	}
	clock := at(2026, 11, 6, 12, 0)
	y := NewSyncer(s, quietLog())
	y.PagePause = 0
	y.Now = func() time.Time { return clock }
	y.SetAccount(src, "korzhov")

	wait := y.step(context.Background())
	if st := y.Status(); st.State != StateError || st.Plays != 6 || st.Total != 10 || wait != 30*time.Second {
		t.Fatalf("после обрыва: %+v, ждать %s", st, wait)
	}
	if m := s.Meta(); m.NextPage != 3 || m.Complete {
		t.Fatalf("продолжение не записано: %+v", m)
	}

	s2, err := Open(path, meta)
	if err != nil {
		t.Fatal(err)
	}
	y2 := NewSyncer(s2, quietLog())
	y2.PagePause = 0
	y2.Now = func() time.Time { return clock }
	y2.SetAccount(src, "korzhov")
	y2.step(context.Background())
	st := y2.Status()
	if st.State != StateReady || st.Plays != 10 {
		t.Fatalf("после продолжения: %+v", st)
	}
	if m := s2.Meta(); !m.Complete || m.SyncedAt != clock.Unix() {
		t.Errorf("загрузка не завершена: %+v", m)
	}

	src.mu.Lock()
	src.plays = append(src.plays, play(clock.Add(10*time.Minute), "Кино", "новый", ""),
		play(clock.Add(-2*24*time.Hour), "Кино", "с телефона", ""))
	src.mu.Unlock()
	clock = clock.Add(15 * time.Minute)
	y2.step(context.Background())
	if n := s2.Len(); n != 11 {
		t.Errorf("быстрая досинхронизация: %d, ожидалось 11 (старый скроббл с телефона — в глубокой)", n)
	}
	clock = clock.Add(7 * time.Hour)
	y2.step(context.Background())
	if n := s2.Len(); n != 12 {
		t.Errorf("глубокая досинхронизация: %d", n)
	}
}

func TestДругойАккаунтСтираетИсторию(t *testing.T) {
	s, _, _ := openTemp(t)
	s.Add([]Play{play(at(2026, 1, 1, 0, 0), "Кино", "трек", "")})
	s.SaveMeta(Meta{User: "старый", Complete: true, SyncedAt: 1})
	src := &fakeSource{perPage: 50, plays: []Play{play(at(2026, 2, 1, 0, 0), "Земфира", "Хочешь?", "")}}
	y := NewSyncer(s, quietLog())
	y.PagePause = 0
	y.SetAccount(src, "новый")
	y.step(context.Background())
	if s.Len() != 1 || s.Meta().User != "новый" {
		t.Errorf("история: %d прослушиваний, пользователь %q", s.Len(), s.Meta().User)
	}
	top, _ := s.Top("artists", "year", 0, at(2026, 6, 1, 0, 0), nil)
	if len(top.Items) != 1 || top.Items[0].Artist != "Земфира" {
		t.Errorf("остались чужие прослушивания: %+v", top.Items)
	}

	y.SetAccount(nil, "")
	if st := y.Status(); st.State != StateOff {
		t.Errorf("без входа: %+v", st)
	}
}

func TestПерезагрузкаИсторииНачинаетЗаново(t *testing.T) {
	s, _, _ := openTemp(t)
	src := &fakeSource{perPage: 2}
	for i := 0; i < 5; i++ {
		src.plays = append(src.plays, play(at(2026, 5, 1, i, 0), "Кино", "трек", ""))
	}
	y := NewSyncer(s, quietLog())
	y.PagePause = 0
	y.SetAccount(src, "korzhov")
	y.step(context.Background())
	if s.Len() != 5 {
		t.Fatalf("загружено %d", s.Len())
	}
	if err := y.Reload(); err != nil {
		t.Fatal(err)
	}
	if s.Len() != 0 || s.Meta().Complete || y.Status().State != StateImporting {
		t.Fatalf("после сброса: %d, %+v, %+v", s.Len(), s.Meta(), y.Status())
	}
	y.step(context.Background())
	if s.Len() != 5 || y.Status().State != StateReady {
		t.Errorf("после перезагрузки: %d, %+v", s.Len(), y.Status())
	}
}

func TestВсегоВИсторииСоСвоимиПрослушиваниями(t *testing.T) {
	s := mustMemory(t)
	s.Add([]Play{play(at(2026, 11, 1, 10, 0), "Кино", "Война", ""), play(at(2026, 11, 2, 10, 0), "Кино", "Война", "")})
	local := []Play{play(at(2026, 11, 2, 10, 0), "Кино", "Война", ""), play(at(2026, 11, 6, 10, 0), "Кино", "Кукушка", "")}
	if n := s.Total(local); n != 3 {
		t.Errorf("всего %d, ожидалось 3", n)
	}
}

func TestТопЗаВсёВремя(t *testing.T) {
	s := mustMemory(t)
	s.Add([]Play{
		play(at(2019, 5, 1, 10, 0), "Кино", "Война", "Группа крови"),
		play(at(2024, 1, 1, 10, 0), "Земфира", "Хочешь?", "Прости меня моя любовь"),
		play(at(2026, 11, 5, 10, 0), "Кино", "Кукушка", "Последний герой"),
	})
	local := []Play{play(at(2026, 11, 6, 10, 0), "Кино", "Война", "Группа крови")}
	now := at(2026, 11, 6, 12, 0)
	top, err := s.Top("artists", "all", -3, now, local)
	if err != nil {
		t.Fatal(err)
	}
	if top.Total != 4 || len(top.Items) != 2 || top.Items[0].Artist != "Кино" || top.Items[0].Count != 3 {
		t.Errorf("всё время: %+v", top)
	}
	p := top.Period
	if p.Label != "за всё время" || p.Offset != 0 || p.Live || p.HasPrev || p.HasNext || p.Since == nil || !p.Since.Equal(at(2019, 5, 1, 10, 0)) {
		t.Errorf("период: %+v", p)
	}
	albums, _ := s.Top("albums", "all", 0, now, local)
	if albums.Items[0].Album != "Группа крови" || albums.Items[0].Count != 2 {
		t.Errorf("альбомы за всё время: %+v", albums.Items)
	}
	empty := mustMemory(t)
	if top, _ := empty.Top("tracks", "all", 0, now, nil); top.Period.Since != nil || len(top.Items) != 0 {
		t.Errorf("пустая история: %+v", top)
	}
}

func TestПоследниеПрослушиванияСНовых(t *testing.T) {
	s := mustMemory(t)
	s.Add([]Play{
		play(at(2026, 11, 2, 10, 0), "Кино", "Война", ""),
		play(at(2026, 11, 6, 10, 0), "Кино", "Кукушка", "Звезда по имени Солнце"),
		play(at(2026, 11, 4, 10, 0), "Земфира", "Хочешь?", ""),
	})
	got := s.Recent(2)
	if len(got) != 2 || got[0].Title != "Кукушка" || got[0].Album != "Звезда по имени Солнце" || got[1].Title != "Хочешь?" {
		t.Errorf("последние: %+v", got)
	}
	if n := len(s.Recent(10)); n != 3 {
		t.Errorf("больше, чем есть: %d", n)
	}
	if n := len(s.Recent(0)); n != 0 {
		t.Errorf("ноль прослушиваний: %d", n)
	}
}

func TestКнопкаОбновитьДосинхронизируетСразу(t *testing.T) {
	s, _, _ := openTemp(t)
	clock := at(2026, 11, 6, 12, 0)
	src := &fakeSource{perPage: 50, plays: []Play{play(clock.Add(-time.Hour), "Кино", "Война", "")}}
	y := NewSyncer(s, quietLog())
	y.PagePause = 0
	y.Now = func() time.Time { return clock }
	y.SetAccount(src, "korzhov")
	y.step(context.Background())
	if s.Len() != 1 {
		t.Fatalf("загружено %d", s.Len())
	}

	src.mu.Lock()
	src.plays = append(src.plays, play(clock.Add(-time.Minute), "Кино", "Кукушка", ""))
	src.mu.Unlock()
	clock = clock.Add(5 * time.Second)
	y.SyncNow()
	y.step(context.Background())
	if s.Len() != 1 {
		t.Errorf("сразу после синхронизации кнопка не дёргает Last.fm снова: %d", s.Len())
	}
	clock = clock.Add(time.Minute)
	y.step(context.Background())
	if s.Len() != 1 {
		t.Errorf("до срока быстрой синхронизации без кнопки: %d", s.Len())
	}
	y.SyncNow()
	y.step(context.Background())
	if s.Len() != 2 {
		t.Errorf("кнопка «обновить» досинхронизирует сразу: %d", s.Len())
	}
}

func TestУдалениеПрослушиванийПереписываетФайл(t *testing.T) {
	s, path, meta := openTemp(t)
	s.Add([]Play{
		play(at(2026, 11, 1, 10, 0), "Кино", "Война", ""),
		play(at(2026, 11, 2, 10, 0), "Кино", "Кукушка", ""),
	})
	if n, err := s.Remove([]Play{play(at(2026, 11, 1, 10, 0), "КИНО", "война", "")}); err != nil || n != 1 {
		t.Fatalf("удалено %d, %v", n, err)
	}
	if n, _ := s.Remove([]Play{play(at(2026, 11, 3, 10, 0), "Кино", "Война", "")}); n != 0 {
		t.Errorf("удалено несуществующее: %d", n)
	}
	again, err := Open(path, meta)
	if err != nil || again.Len() != 1 || again.Recent(1)[0].Title != "Кукушка" {
		t.Errorf("после перезапуска: %d, %v", again.Len(), err)
	}
	if n, _ := s.Add([]Play{play(at(2026, 11, 1, 10, 0), "Кино", "Война", "")}); n != 1 {
		t.Errorf("удалённое можно добавить снова: %d", n)
	}
}

func TestМестоТекущегоТрекаВТопах(t *testing.T) {
	s := mustMemory(t)
	now := at(2026, 11, 6, 12, 0)
	var plays []Play
	for i := 0; i < 5; i++ {
		plays = append(plays, play(now.Add(-time.Duration(i+1)*time.Hour), "Кино", "Кукушка", "Звезда по имени Солнце"))
	}
	for i := 0; i < 3; i++ {
		plays = append(plays, play(now.Add(-time.Duration(i+10)*time.Hour), "Земфира", "Хочешь?", "ПММЛ"))
		plays = append(plays, play(at(2026, 3, 1, 10, i), "Земфира", "Искала", "ПММЛ"))
		plays = append(plays, play(at(2026, 3, 2, 10, i), "Земфира", "Хочешь?", "ПММЛ"))
	}
	s.Add(plays)

	got := map[string]int{}
	for _, r := range s.Ranks("кино", "кукушка", "звезда по имени солнце", now, nil) {
		got[r.Kind+"/"+r.Period] = r.Rank
	}
	want := map[string]int{"track/week": 1, "track/month": 1, "track/year": 2, "artist/week": 1, "artist/month": 1, "artist/year": 2,
		"album/week": 1, "album/month": 1, "album/year": 2}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: место %d, ожидалось %d (все: %v)", k, got[k], w, got)
		}
	}
	z := map[string]int{}
	for _, r := range s.Ranks("Земфира", "Хочешь?", "ПММЛ", now, nil) {
		z[r.Kind+"/"+r.Period] = r.Rank
	}
	if z["track/year"] != 1 || z["track/week"] != 2 || z["artist/year"] != 1 || z["album/year"] != 1 {
		t.Errorf("места Земфиры: %v", z)
	}
	if r := s.Ranks("Сплин", "Выхода нет", "", now, nil); len(r) != 0 {
		t.Errorf("неслушанный трек не в топе: %v", r)
	}
	extra := []Play{play(now.Add(-30*time.Minute), "Сплин", "Выхода нет", "")}
	if r := s.Ranks("Сплин", "Выхода нет", "", now, extra); len(r) != 6 {
		t.Errorf("свои прослушивания учитываются: %v", r)
	}
}

func TestСколькоРазСлушалиКандидатов(t *testing.T) {
	s, err := Open("", "")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var plays []Play
	for i := 0; i < 5; i++ {
		plays = append(plays, Play{At: base.Add(time.Duration(i) * time.Hour), Artist: "Первая группа", Title: "Песня один"})
	}
	plays = append(plays, Play{At: base.Add(10 * time.Hour), Artist: "Первая группа", Title: "Песня два"})
	plays = append(plays, Play{At: base.Add(11 * time.Hour), Artist: "Вторая группа", Title: "Другая"})
	if _, err := s.Add(plays); err != nil {
		t.Fatal(err)
	}
	extra := []Play{{At: base.Add(20 * time.Hour), Artist: "первая  ГРУППА", Title: "песня ОДИН"}}
	ac, tc := s.PlayCounts(
		[]string{"Первая группа", "Новая группа", "вторая группа"},
		[]Play{{Artist: "Первая группа", Title: "Песня один"}, {Artist: "Новая группа", Title: "Песня один"}, {Artist: "Первая группа", Title: "Песня два"}},
		extra)
	if len(ac) != 3 || ac[0] != 7 || ac[1] != 0 || ac[2] != 1 {
		t.Errorf("прослушивания исполнителей: %v", ac)
	}
	if len(tc) != 3 || tc[0] != 6 || tc[1] != 0 || tc[2] != 1 {
		t.Errorf("прослушивания треков: %v", tc)
	}
}
