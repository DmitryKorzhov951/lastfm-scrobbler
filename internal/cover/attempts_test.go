package cover

import (
	"image/color"
	"testing"
	"time"
)

func TestОбложкаПлеераПеречитываетсяПослеСменыТрека(t *testing.T) {
	a := NewAttempts()
	old, fresh := pngBytes(t, color.RGBA{1, 1, 1, 255}), pngBytes(t, color.RGBA{2, 2, 2, 255})
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	start := now
	const k = "Spotify.exe\x00Charli xcx\x00360\x00BRAT"

	var reads []time.Duration
	for i := 0; i < 10; i++ {
		if a.Due(k, now) {
			reads = append(reads, now.Sub(start))
			data := old
			if len(reads) > 1 {
				data = fresh
			}
			a.Put(k, data, now)
		}
		a.Sweep()
		now = now.Add(2 * time.Second)
	}
	want := []time.Duration{0, 2 * time.Second, 6 * time.Second}
	if len(reads) != len(want) || reads[0] != want[0] || reads[1] != want[1] || reads[2] != want[2] {
		t.Fatalf("чтения на %v, ожидались на %v", reads, want)
	}
	if string(a.Get(k)) != string(fresh) {
		t.Error("остаётся последняя прочитанная обложка — плеер мог сначала отдать картинку прошлого трека")
	}
}

func TestНеудачноеЧтениеНеСтираетОбложку(t *testing.T) {
	a := NewAttempts()
	now := time.Now()
	art := pngBytes(t, color.RGBA{3, 3, 3, 255})
	a.Due("k", now)
	a.Put("k", art, now)
	now = now.Add(attemptsRetry)
	if !a.Due("k", now) {
		t.Fatal("второе чтение по расписанию")
	}
	a.Put("k", nil, now)
	a.Put("k", []byte("не картинка"), now)
	if string(a.Get("k")) != string(art) {
		t.Error("пустое чтение и не картинка не стирают прочитанную раньше обложку")
	}
}

func TestОбложкиЗакрытыхВкладокЗабываются(t *testing.T) {
	a := NewAttempts()
	now := time.Now()
	one, two := pngBytes(t, color.RGBA{4, 4, 4, 255}), pngBytes(t, color.RGBA{5, 5, 5, 255})
	a.Due("chrome\x00Вкладка 1", now)
	a.Put("chrome\x00Вкладка 1", one, now)
	a.Due("chrome\x00Вкладка 2", now)
	a.Put("chrome\x00Вкладка 2", two, now)
	a.Sweep()
	a.Due("chrome\x00Вкладка 1", now)
	a.Sweep()
	if a.Get("chrome\x00Вкладка 2") != nil || a.Get("chrome\x00Вкладка 1") == nil {
		t.Error("обложка пропавшей сессии удаляется, живой — остаётся")
	}
}
