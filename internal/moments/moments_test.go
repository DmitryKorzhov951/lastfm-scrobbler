package moments

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

const (
	artist = "Тестовая группа"
	title  = "Песня"
	length = 200.0
)

type player struct {
	s   *Store
	now time.Time
}

func newPlayer(t *testing.T) (*player, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "moments.json")
	s, err := Open(path, quiet)
	if err != nil {
		t.Fatal(err)
	}
	return &player{s: s, now: time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)}, path
}

func (p *player) at(pos float64, playing bool) {
	p.s.Observe([]Sample{{Player: "spotify", Artist: artist, Title: title, Length: length, Position: pos, Playing: playing}}, p.now)
	p.now = p.now.Add(time.Second)
}

func (p *player) play(from, to float64) {
	for pos := from; pos <= to; pos++ {
		p.at(pos, true)
	}
}

func (p *player) stop() {
	p.s.Observe(nil, p.now)
	p.now = p.now.Add(time.Second)
}

func TestОбычноеПрослушиваниеНеДаётМомента(t *testing.T) {
	p, path := newPlayer(t)
	for i := 0; i < 5; i++ {
		p.play(0, length)
		p.stop()
	}
	for pos := 0.4; pos < 60; pos++ {
		p.at(pos, true)
	}
	p.stop()
	if m, ok := p.s.Moment(artist, title, length); ok {
		t.Errorf("песню просто слушали от начала — момента нет: %+v", m)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("без переслушиваний файл не пишется: %v", err)
	}
}

func TestПереслушанноеМестоСтановитсяМоментомСразу(t *testing.T) {
	p, path := newPlayer(t)
	p.play(0, 70)
	p.play(40, 55)
	if m, ok := p.s.Moment(artist, title, length); !ok || m.Start < 39 || m.Start > 42 || m.End > 57 {
		t.Fatalf("момент виден, пока песня ещё играет: %+v, %v", m, ok)
	}
	p.play(56, 90)
	p.stop()
	m, ok := p.s.Moment(artist, title, length)
	if !ok || m.Manual || m.Start < 39 || m.Start > 42 || m.End < 69 || m.End > 72 {
		t.Fatalf("момент на переслушанном месте 40–70: %+v, %v", m, ok)
	}
	again, err := Open(path, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if m2, ok := again.Moment(artist, title, length); !ok || m2 != m {
		t.Errorf("после перезапуска момент тот же: %+v, %v", m2, ok)
	}
}

func TestКороткуюПеремоткуНадоПовторить(t *testing.T) {
	p, _ := newPlayer(t)
	p.play(0, 100)
	p.play(95, 120)
	p.stop()
	if m, ok := p.s.Moment(artist, title, length); ok {
		t.Errorf("переслушать 5 секунд один раз мало: %+v", m)
	}
	p.play(0, 100)
	p.play(95, 120)
	p.stop()
	if m, ok := p.s.Moment(artist, title, length); !ok || m.Start > 96 || m.End < 99 {
		t.Errorf("дважды — уже момент: %+v, %v", m, ok)
	}
}

func TestНачалоТрекаНеСтановитсяМоментом(t *testing.T) {
	p, _ := newPlayer(t)
	for i := 0; i < 6; i++ {
		p.play(0, 50)
		p.play(2, 50)
		p.play(0, 11)
		p.play(1, 30)
		p.stop()
	}
	if m, ok := p.s.Moment(artist, title, length); ok {
		t.Errorf("перезапуск песни с начала — не переслушивание: %+v", m)
	}
	for i := 0; i < 4; i++ {
		p.play(0, 40)
		p.play(13, 40)
		p.stop()
	}
	m, ok := p.s.Moment(artist, title, length)
	if !ok || m.Start < StartZone {
		t.Errorf("момент не раньше %v с: %+v, %v", StartZone, m, ok)
	}
}

func TestПрыжокВпередОтмечаетМесто(t *testing.T) {
	p, _ := newPlayer(t)
	for i := 0; i < 3; i++ {
		p.play(0, 20)
		p.play(120, 150)
		p.stop()
	}
	m, ok := p.s.Moment(artist, title, length)
	if !ok || m.Start < 118 || m.Start > 122 || m.End > 140 {
		t.Errorf("три прыжка на 2:00: %+v, %v", m, ok)
	}
}

func TestПаузаНеСчитаетсяПрослушиванием(t *testing.T) {
	p, _ := newPlayer(t)
	for i := 0; i < 3; i++ {
		p.play(0, 60)
		for j := 0; j < 30; j++ {
			p.at(60, false)
		}
		p.at(30, false)
		p.play(30, 80)
		p.stop()
	}
	m, ok := p.s.Moment(artist, title, length)
	if !ok || m.Start < 29 || m.Start > 32 || m.End < 58 || m.End > 62 {
		t.Errorf("перемотка на паузе: %+v, %v", m, ok)
	}
}

func TestСвойМоментВажнееАвтоматического(t *testing.T) {
	p, path := newPlayer(t)
	for i := 0; i < 3; i++ {
		p.play(0, 70)
		p.play(40, 90)
		p.stop()
	}
	auto, ok := p.s.Moment(artist, title, length)
	if !ok {
		t.Fatal("нет автоматического момента")
	}
	if err := p.s.SetManual(" тестовая  ГРУППА", "песня", length, Moment{Start: 100.04, End: 260}, p.now); err != nil {
		t.Fatal(err)
	}
	m, ok := p.s.Moment(artist, title, length)
	if !ok || !m.Manual || m.Start != 100 || m.End != length {
		t.Errorf("свой момент: %+v, %v", m, ok)
	}
	reopened, _ := Open(path, quiet)
	if m2, _ := reopened.Moment(artist, title, length); m2 != m {
		t.Errorf("свой момент сохраняется: %+v", m2)
	}
	if err := p.s.ClearManual(artist, title); err != nil {
		t.Fatal(err)
	}
	if m, _ := p.s.Moment(artist, title, length); m != auto {
		t.Errorf("после сброса снова автоматический: %+v, ожидался %+v", m, auto)
	}
	for _, bad := range []Moment{{Start: -1, End: 10}, {Start: 50, End: 50.5}, {Start: 199.5, End: 260}, {Start: 250, End: 260}} {
		if err := p.s.SetManual(artist, title, length, bad, p.now); err == nil {
			t.Errorf("неверный момент принят: %+v", bad)
		}
	}
	if err := p.s.SetManual(artist, title, 0, Moment{Start: 1, End: 10}, p.now); err == nil {
		t.Error("без длины трека момент не задаётся")
	}
	if err := p.s.SetManual("", title, length, Moment{Start: 1, End: 10}, p.now); err == nil {
		t.Error("без исполнителя момент не задаётся")
	}
}

func TestСвойМоментБезПереслушиванийУдаляетсяЦеликом(t *testing.T) {
	p, path := newPlayer(t)
	if err := p.s.SetManual(artist, title, length, Moment{Start: 30, End: 45}, p.now); err != nil {
		t.Fatal(err)
	}
	if err := p.s.ClearManual(artist, title); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.s.Moment(artist, title, length); ok {
		t.Error("после сброса момента нет")
	}
	reopened, _ := Open(path, quiet)
	if len(reopened.tracks) != 0 {
		t.Errorf("пустая запись о треке не хранится: %d", len(reopened.tracks))
	}
}

func TestДвеПесниИДваПлеераОтдельно(t *testing.T) {
	p, _ := newPlayer(t)
	other := func(pos float64) Sample {
		return Sample{Player: "aimp", Artist: "Другая группа", Title: "Другая песня", Length: 180, Position: pos, Playing: true}
	}
	for i := 0; i < 3; i++ {
		for pos := 0.0; pos <= 70; pos++ {
			p.s.Observe([]Sample{
				{Player: "spotify", Artist: artist, Title: title, Length: length, Position: pos, Playing: true},
				other(pos + 100),
			}, p.now)
			p.now = p.now.Add(time.Second)
		}
		for pos := 40.0; pos <= 80; pos++ {
			p.s.Observe([]Sample{
				{Player: "spotify", Artist: artist, Title: title, Length: length, Position: pos, Playing: true},
				other(pos + 131),
			}, p.now)
			p.now = p.now.Add(time.Second)
		}
		p.stop()
	}
	if _, ok := p.s.Moment(artist, title, length); !ok {
		t.Error("у первой песни момент есть")
	}
	if m, ok := p.s.Moment("Другая группа", "Другая песня", 180); ok {
		t.Errorf("вторую песню играли подряд — у неё момента нет: %+v", m)
	}
}
