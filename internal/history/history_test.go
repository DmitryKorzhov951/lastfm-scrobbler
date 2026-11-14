package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func entry(id int64, title string, started time.Time, secs int) Entry {
	return Entry{ID: id, Artist: "Charli XCX", Title: title, Seconds: secs, Player: "spotify", Started: started}
}

func TestИтогиЗаСегодняИЗаВсёВремя(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	s, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	loc := time.FixedZone("MSK", 3*3600)
	now := time.Date(2026, 10, 18, 18, 45, 0, 0, loc)
	yesterday := now.Add(-24 * time.Hour)

	for _, e := range []Entry{
		entry(1, "360", yesterday, 133),
		entry(2, "Von dutch", now.Add(-40*time.Minute), 164),
		entry(3, "Apple", now.Add(-20*time.Minute), 151),
		entry(4, "Guess", time.Date(2026, 10, 18, 0, 0, 30, 0, loc), 145),
	} {
		if err := s.Add(e); err != nil {
			t.Fatal(err)
		}
	}

	tot := s.Totals(now)
	if tot.TodayCount != 3 || tot.TodayTime != (164+151+145)*time.Second {
		t.Errorf("сегодня: %d, %s", tot.TodayCount, tot.TodayTime)
	}
	if tot.AllCount != 4 || tot.AllTime != (133+164+151+145)*time.Second {
		t.Errorf("всего: %d, %s", tot.AllCount, tot.AllTime)
	}

	if err := s.SetStatus(Ignored, "Timestamp too old", 3); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(Sent, "", 2); err != nil {
		t.Fatal(err)
	}
	tot = s.Totals(now)
	if tot.TodayCount != 2 || tot.AllCount != 3 {
		t.Errorf("после отклонения: сегодня %d, всего %d", tot.TodayCount, tot.AllCount)
	}

	if err := s.Delete(4); err != nil {
		t.Fatal(err)
	}
	if got := s.Totals(now); got.AllCount != 2 || got.TodayCount != 1 {
		t.Errorf("после удаления: %+v", got)
	}

	s2, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	recent := s2.Recent(10)
	if len(recent) != 3 || recent[0].Title != "Apple" || recent[2].Title != "360" {
		t.Fatalf("недавние, новые первыми: %+v", recent)
	}
	if recent[0].Status != Ignored || recent[0].Reason != "Timestamp too old" || recent[1].Status != Sent {
		t.Errorf("статусы не сохранились: %+v", recent)
	}
	if s2.LastID() != 4 {
		t.Errorf("LastID = %d, ожидалось 4", s2.LastID())
	}
	if got := s2.Totals(now); got.AllCount != 2 {
		t.Errorf("итог после перезапуска = %d", got.AllCount)
	}
}

func TestПределИсторииНеТрогаетИтоги(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "history.json"), 3)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i := int64(1); i <= 5; i++ {
		if err := s.Add(entry(i, "t", now, 100)); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(s.Recent(10)); got != 3 {
		t.Errorf("подробных записей %d, ожидалось 3", got)
	}
	if got := s.Totals(now).AllCount; got != 5 {
		t.Errorf("итог за всё время = %d, ожидалось 5", got)
	}
	if err := s.Delete(1); err != nil {
		t.Fatal(err)
	}
	if got := s.Totals(now).AllCount; got != 5 {
		t.Errorf("итог не должен меняться: %d", got)
	}
}

func TestЗамеченныеПлееры(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	s, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range [][2]string{{"spotify", "Spotify"}, {"aimp", "AIMP"}, {"spotify", "Spotify"}, {"firefox", ""}} {
		if err := s.SeePlayer(p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	s2, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := s2.Players()
	want := []Player{{"aimp", "AIMP"}, {"firefox", "firefox"}, {"spotify", "Spotify"}}
	if len(got) != len(want) {
		t.Fatalf("плееры = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("плеер %d = %+v, ожидалось %+v", i, got[i], want[i])
		}
	}
}

func TestПовреждённаяИстория(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(path, []byte("{не json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path, 0)
	if err == nil || s == nil {
		t.Fatalf("ожидалось предупреждение и рабочая история: %v, %v", s, err)
	}
	if _, err := os.Stat(path + ".broken"); err != nil {
		t.Errorf("повреждённый файл не сохранён: %v", err)
	}
	if err := s.Add(entry(1, "a", time.Now(), 60)); err != nil {
		t.Error(err)
	}
}

func TestЗаписиНачиная(t *testing.T) {
	s, err := Open("", 0)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		s.Add(entry(int64(i+1), "трек", base.Add(time.Duration(i)*24*time.Hour), 100))
	}
	got := s.Since(base.Add(48 * time.Hour))
	if len(got) != 3 || got[0].ID != 3 || got[2].ID != 5 {
		t.Errorf("с 3 ноября: %+v", got)
	}
	if n := len(s.Since(time.Time{})); n != 5 {
		t.Errorf("все записи: %d", n)
	}
}
