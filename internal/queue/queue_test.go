package queue

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lastfm"
)

func scrobble(title string, ts time.Time) lastfm.Scrobble {
	return lastfm.Scrobble{
		Track:     lastfm.Track{Artist: "Артист", Title: title, Duration: 3 * time.Minute},
		Timestamp: ts,
	}
}

func TestОчередьПереживаетПерезапуск(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q, err := Open(path, 0)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if q.Len() != 0 {
		t.Fatalf("новая очередь не пуста: %d", q.Len())
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, title := range []string{"Первый", "Второй", "Третий"} {
		if err := q.Add(scrobble(title, now)); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	q2, err := Open(path, 0)
	if err != nil {
		t.Fatalf("повторный Open: %v", err)
	}
	if q2.Len() != 3 {
		t.Fatalf("после перезапуска в очереди %d записей, ожидалось 3", q2.Len())
	}
	items := q2.Items()
	if items[0].Scrobble.Title != "Первый" || items[2].Scrobble.Title != "Третий" {
		t.Errorf("порядок записей нарушен: %+v", items)
	}
	if !items[0].Scrobble.Timestamp.Equal(now) {
		t.Errorf("метка времени = %s, ожидалось %s", items[0].Scrobble.Timestamp, now)
	}
	if items[0].Scrobble.Duration != 3*time.Minute {
		t.Errorf("длительность не сохранилась: %s", items[0].Scrobble.Duration)
	}
}

func TestHeadИDrop(t *testing.T) {
	q, err := Open(filepath.Join(t.TempDir(), "queue.json"), 0)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, title := range []string{"a", "b", "c", "d"} {
		if err := q.Add(scrobble(title, now)); err != nil {
			t.Fatal(err)
		}
	}

	head := q.Head(2)
	if len(head) != 2 || head[0].Scrobble.Title != "a" || head[1].Scrobble.Title != "b" {
		t.Fatalf("Head(2) = %+v", head)
	}
	head[0].Attempts = 99
	if q.Items()[0].Attempts != 0 {
		t.Error("Head должен возвращать копию записей")
	}

	if err := q.Drop(2); err != nil {
		t.Fatal(err)
	}
	if q.Len() != 2 || q.Items()[0].Scrobble.Title != "c" {
		t.Fatalf("после Drop(2) очередь = %+v", q.Items())
	}

	if len(q.Head(100)) != 2 {
		t.Error("Head(100) должен вернуть всё, что есть")
	}
	if err := q.Drop(100); err != nil {
		t.Fatal(err)
	}
	if q.Len() != 0 {
		t.Error("очередь должна опустеть")
	}
}

func TestMarkAttemptИClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := q.Add(scrobble("a", now)); err != nil {
		t.Fatal(err)
	}
	if err := q.MarkAttempt(1); err != nil {
		t.Fatal(err)
	}
	if err := q.MarkAttempt(1); err != nil {
		t.Fatal(err)
	}

	q2, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := q2.Items()[0].Attempts; got != 2 {
		t.Errorf("попыток = %d, ожидалось 2", got)
	}

	if err := q2.Clear(); err != nil {
		t.Fatal(err)
	}
	q3, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if q3.Len() != 0 {
		t.Error("после Clear очередь должна быть пуста и на диске")
	}
}

func TestПределРазмераОчереди(t *testing.T) {
	q, err := Open(filepath.Join(t.TempDir(), "queue.json"), 3)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, title := range []string{"1", "2", "3", "4", "5"} {
		if err := q.Add(scrobble(title, now)); err != nil {
			t.Fatal(err)
		}
	}
	if q.Len() != 3 {
		t.Fatalf("длина = %d, ожидалось 3", q.Len())
	}
	if got := q.Items()[0].Scrobble.Title; got != "3" {
		t.Errorf("первая запись = %q, ожидалось \"3\"", got)
	}
}

func TestПоврежденныйФайлОтводитсяВСторону(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "queue.json")
	if err := os.WriteFile(path, []byte("{это не json"), 0o600); err != nil {
		t.Fatal(err)
	}

	q, err := Open(path, 0)
	if err == nil {
		t.Fatal("ожидалось предупреждение о повреждённом файле")
	}
	if q == nil {
		t.Fatal("даже при повреждённом файле очередь должна быть рабочей")
	}
	if _, err := os.Stat(path + ".broken"); err != nil {
		t.Errorf("повреждённый файл не сохранён: %v", err)
	}
	if err := q.Add(scrobble("a", time.Now().UTC())); err != nil {
		t.Errorf("запись в новую очередь: %v", err)
	}
}

func TestПраваДоступаКФайлу(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("права в стиле Unix в Windows не применяются; %LOCALAPPDATA% закрыт для других пользователей")
	}
	path := filepath.Join(t.TempDir(), "sub", "queue.json")
	q, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Add(scrobble("a", time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("права файла очереди = %o, ожидалось 600", perm)
	}
}

func TestНомераЗаписейИУдалениеПоНомеру(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	var ids []int64
	for _, title := range []string{"a", "b", "c", "d"} {
		it, err := q.Push(scrobble(title, now))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, it.ID)
	}
	if ids[0] == 0 || ids[1] != ids[0]+1 || ids[3] != ids[0]+3 {
		t.Fatalf("номера должны идти подряд с единицы: %v", ids)
	}

	head := q.Head(2)
	if n, err := q.Remove(ids[2]); err != nil || n != 1 {
		t.Fatalf("Remove(c) = %d, %v", n, err)
	}
	if err := q.MarkAttempts(ids[3]); err != nil {
		t.Fatal(err)
	}
	if n, err := q.Remove(head[0].ID, head[1].ID); err != nil || n != 2 {
		t.Fatalf("Remove(a, b) = %d, %v", n, err)
	}
	items := q.Items()
	if len(items) != 1 || items[0].Scrobble.Title != "d" || items[0].Attempts != 1 {
		t.Fatalf("в очереди должна остаться d с одной попыткой: %+v", items)
	}
	if n, err := q.Remove(ids[0]); err != nil || n != 0 {
		t.Fatalf("повторный Remove = %d, %v", n, err)
	}

	q2, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := q2.Items()[0].ID; got != ids[3] {
		t.Errorf("номер после перезапуска = %d, ожидалось %d", got, ids[3])
	}
	it, err := q2.Push(scrobble("e", now))
	if err != nil {
		t.Fatal(err)
	}
	if it.ID != ids[3]+1 {
		t.Errorf("номер новой записи = %d, ожидалось %d", it.ID, ids[3]+1)
	}
}

func TestСтарыйФайлБезНомеров(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	old := `[{"scrobble":{"track":{"artist":"A","track":"x"},"timestamp":"2026-10-01T10:00:00Z"},"queued_at":"2026-10-01T10:00:00Z","attempts":1},
	         {"scrobble":{"track":{"artist":"A","track":"y"},"timestamp":"2026-10-01T10:04:00Z"},"queued_at":"2026-10-01T10:04:00Z","attempts":0}]`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	q, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	items := q.Items()
	if items[0].ID != 1 || items[1].ID != 2 {
		t.Fatalf("записи прежней версии должны получить номера 1 и 2: %+v", items)
	}
	q.SetLastID(40)
	it, err := q.Push(scrobble("z", time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	if it.ID != 41 {
		t.Errorf("после SetLastID(40) номер = %d, ожидалось 41", it.ID)
	}
}
