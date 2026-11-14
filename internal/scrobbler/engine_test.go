package scrobbler

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lastfm"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

type fakeSink struct {
	nowPlaying []lastfm.Track
	scrobbles  []lastfm.Scrobble
}

func (f *fakeSink) NowPlaying(t lastfm.Track)  { f.nowPlaying = append(f.nowPlaying, t) }
func (f *fakeSink) Scrobble(s lastfm.Scrobble) { f.scrobbles = append(f.scrobbles, s) }

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

type sim struct {
	t      *testing.T
	engine *Engine
	sink   *fakeSink
	now    time.Time
	id     string
	name   string
	track  media.Track
	pos    time.Duration
	status media.Status

	knownPos   bool
	reportedAt time.Time
}

func newSim(t *testing.T, opts Options, id string) *sim {
	t.Helper()
	sink := &fakeSink{}
	return &sim{
		t:      t,
		engine: New(opts, sink, quietLogger()),
		sink:   sink,
		now:    time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC),
		id:     id,
		name:   shortName(id),
	}
}

func (s *sim) state() media.State {
	return media.State{
		ID:       s.id,
		Name:     s.name,
		Identity: s.name,
		Status:   s.status,
		Track:    s.track,
		Position: s.pos,
		At:       s.now,

		PositionKnown:    s.knownPos,
		PositionReported: s.reportedAt,
	}
}

func (s *sim) play(tr media.Track) {
	s.track = tr
	s.pos = 0
	s.status = media.Playing
	s.engine.Update(s.state())
}

func (s *sim) advance(d time.Duration) {
	for left, n := d, 0; left > 0; left, n = left-time.Second, n+1 {
		s.now = s.now.Add(time.Second)
		if s.status == media.Playing {
			s.pos += time.Second
		}
		if n%5 == 4 {
			s.engine.Update(s.state())
		} else {
			s.engine.Tick(s.now)
		}
	}
}

func (s *sim) tick(d time.Duration) {
	for left := d; left > 0; left -= time.Second {
		s.now = s.now.Add(time.Second)
		s.engine.Tick(s.now)
	}
}

func (s *sim) pause() {
	s.status = media.Paused
	s.engine.Update(s.state())
}

func (s *sim) resume() {
	s.status = media.Playing
	s.engine.Update(s.state())
}

func (s *sim) stop() {
	s.status = media.Stopped
	s.engine.Update(s.state())
}

func (s *sim) seek(pos time.Duration) {
	s.pos = pos
	s.engine.Update(s.state())
}

func shortName(id string) string {
	return strings.TrimSuffix(strings.ToLower(id), ".exe")
}

func track(artist, title string, length time.Duration) media.Track {
	return media.Track{
		Title:   title,
		Artists: []string{artist},
		Album:   "Альбом",
		Length:  length,
	}
}

func TestСкробблНаПоловинеТрека(t *testing.T) {
	s := newSim(t, DefaultOptions(), "vlc.exe")
	s.play(track("Radiohead", "Creep", 4*time.Minute))

	s.advance(119 * time.Second)
	if len(s.sink.scrobbles) != 0 {
		t.Fatalf("скроббл раньше половины трека: %+v", s.sink.scrobbles)
	}

	s.advance(2 * time.Second)
	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("ожидался один скроббл, получено %d", len(s.sink.scrobbles))
	}
	got := s.sink.scrobbles[0]
	if got.Artist != "Radiohead" || got.Title != "Creep" {
		t.Errorf("отправлен неверный трек: %+v", got.Track)
	}
	if got.Duration != 4*time.Minute {
		t.Errorf("длительность = %s", got.Duration)
	}

	s.advance(2 * time.Minute)
	if len(s.sink.scrobbles) != 1 {
		t.Errorf("трек засчитан повторно: %d", len(s.sink.scrobbles))
	}
}

func TestДлинныйТрекЗасчитываетсяЧерезЧетыреМинуты(t *testing.T) {
	s := newSim(t, DefaultOptions(), "mpv.exe")
	s.play(track("Pink Floyd", "Echoes", 23*time.Minute))

	s.advance(3*time.Minute + 50*time.Second)
	if len(s.sink.scrobbles) != 0 {
		t.Fatal("скроббл раньше четырёх минут")
	}
	s.advance(15 * time.Second)
	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("длинный трек должен засчитываться через 4 минуты, получено %d", len(s.sink.scrobbles))
	}
}

func TestКороткийТрекНеЗасчитывается(t *testing.T) {
	s := newSim(t, DefaultOptions(), "vlc.exe")
	s.play(track("Napalm Death", "You Suffer", 2*time.Second))
	s.advance(time.Minute)
	if len(s.sink.scrobbles) != 0 {
		t.Errorf("трек короче 30 секунд не должен скроббиться: %+v", s.sink.scrobbles)
	}
}

func TestПаузаНеНакапливаетВремя(t *testing.T) {
	s := newSim(t, DefaultOptions(), "vlc.exe")
	s.play(track("Portishead", "Roads", 4*time.Minute))

	s.advance(60 * time.Second)
	s.pause()
	s.advance(10 * time.Minute)
	if len(s.sink.scrobbles) != 0 {
		t.Fatal("во время паузы трек не должен засчитываться")
	}

	s.resume()
	s.advance(59 * time.Second)
	if len(s.sink.scrobbles) != 0 {
		t.Fatal("скроббл раньше порога после паузы")
	}
	s.advance(2 * time.Second)
	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("после паузы трек должен досчитаться, получено %d", len(s.sink.scrobbles))
	}
}

func TestПеремоткаВпередБезПозицииПлеераНеЗасчитываетТрек(t *testing.T) {
	s := newSim(t, DefaultOptions(), "vlc.exe")
	s.play(track("Massive Attack", "Teardrop", 5*time.Minute))

	s.advance(30 * time.Second)
	s.seek(4 * time.Minute)
	s.advance(20 * time.Second)

	if len(s.sink.scrobbles) != 0 {
		t.Error("перемотка не должна засчитывать неслушанное время")
	}
}

func TestПовторноеВоспроизведениеТогоЖеТрека(t *testing.T) {
	s := newSim(t, DefaultOptions(), "vlc.exe")
	tr := track("Nirvana", "Lithium", 4*time.Minute)
	s.play(tr)
	s.advance(2 * time.Minute)
	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("первый скроббл не сделан: %d", len(s.sink.scrobbles))
	}

	s.seek(0)
	s.advance(2*time.Minute + time.Second)
	if len(s.sink.scrobbles) != 2 {
		t.Fatalf("повторное прослушивание должно засчитываться, получено %d", len(s.sink.scrobbles))
	}
	if !s.sink.scrobbles[1].Timestamp.After(s.sink.scrobbles[0].Timestamp) {
		t.Error("у повторного прослушивания должна быть более поздняя метка времени")
	}
}

func TestСменаТрекаСбрасываетСчётчик(t *testing.T) {
	s := newSim(t, DefaultOptions(), "vlc.exe")
	s.play(track("A", "Первый", 4*time.Minute))
	s.advance(90 * time.Second)

	s.play(track("A", "Второй", 4*time.Minute))
	s.advance(90 * time.Second)
	if len(s.sink.scrobbles) != 0 {
		t.Fatalf("ни один трек не дослушан до порога: %+v", s.sink.scrobbles)
	}

	s.advance(31 * time.Second)
	if len(s.sink.scrobbles) != 1 || s.sink.scrobbles[0].Title != "Второй" {
		t.Fatalf("засчитан не тот трек: %+v", s.sink.scrobbles)
	}
}

func TestМеткаВремениЭтоНачалоТрека(t *testing.T) {
	s := newSim(t, DefaultOptions(), "vlc.exe")
	start := s.now
	s.play(track("A", "B", 4*time.Minute))
	s.advance(2 * time.Minute)

	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("ожидался скроббл, получено %d", len(s.sink.scrobbles))
	}
	got := s.sink.scrobbles[0].Timestamp
	if !got.Equal(start.UTC()) {
		t.Errorf("метка времени = %s, ожидалось начало трека %s", got, start.UTC())
	}
}

func TestПодхватУжеИграющегоТрека(t *testing.T) {
	s := newSim(t, DefaultOptions(), "vlc.exe")
	start := s.now
	s.track = track("A", "B", 4*time.Minute)
	s.pos = 60 * time.Second
	s.status = media.Playing
	s.engine.Update(s.state())

	s.advance(61 * time.Second)
	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("ожидался скроббл, получено %d", len(s.sink.scrobbles))
	}
	want := start.Add(-60 * time.Second).UTC()
	if got := s.sink.scrobbles[0].Timestamp; !got.Equal(want) {
		t.Errorf("метка времени = %s, ожидалось %s", got, want)
	}
}

func TestСейчасИграетОтправляетсяОдинРаз(t *testing.T) {
	s := newSim(t, DefaultOptions(), "vlc.exe")
	s.play(track("A", "B", 4*time.Minute))
	s.advance(30 * time.Second)

	if len(s.sink.nowPlaying) != 1 {
		t.Fatalf("«сейчас играет» отправлено %d раз, ожидался 1", len(s.sink.nowPlaying))
	}
	s.advance(4 * time.Minute)
	if len(s.sink.nowPlaying) != 2 {
		t.Errorf("уведомление должно обновляться каждые 4 минуты, получено %d", len(s.sink.nowPlaying))
	}
}

func TestТрекБезМетаданныхИгнорируется(t *testing.T) {
	s := newSim(t, DefaultOptions(), "vlc.exe")
	s.play(media.Track{Title: "", Length: 4 * time.Minute})
	s.advance(3 * time.Minute)
	if len(s.sink.scrobbles) != 0 || len(s.sink.nowPlaying) != 0 {
		t.Error("без артиста и названия трек отправлять нельзя")
	}
}

func TestПотокБезДлительности(t *testing.T) {
	s := newSim(t, DefaultOptions(), "mpv.exe")
	s.play(media.Track{Title: "Песня", Artists: []string{"Радио"}})
	s.advance(3*time.Minute + 50*time.Second)
	if len(s.sink.scrobbles) != 0 {
		t.Fatal("поток засчитан раньше четырёх минут")
	}
	s.advance(15 * time.Second)
	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("поток должен засчитываться через 4 минуты, получено %d", len(s.sink.scrobbles))
	}
}

func TestПотокМожноОтключить(t *testing.T) {
	opts := DefaultOptions()
	opts.SkipUnknownLength = true
	s := newSim(t, opts, "mpv.exe")
	s.play(media.Track{Title: "Песня", Artists: []string{"Радио"}})
	s.advance(10 * time.Minute)
	if len(s.sink.scrobbles) != 0 {
		t.Error("при skip_unknown_length потоки скроббиться не должны")
	}
}

func TestСчитаетсяТолькоАктивныйПлеер(t *testing.T) {
	sink := &fakeSink{}
	engine := New(DefaultOptions(), sink, quietLogger())
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	trA := track("A", "Первый", 4*time.Minute)
	trB := track("B", "Второй", 4*time.Minute)

	engine.Update(media.State{ID: "bus.a", Name: "a", Status: media.Playing, Track: trA, At: now})
	now = now.Add(10 * time.Second)
	engine.Update(media.State{ID: "bus.b", Name: "b", Status: media.Playing, Track: trB, At: now})

	for i := 0; i < 130; i++ {
		now = now.Add(time.Second)
		engine.Tick(now)
	}

	if len(sink.scrobbles) != 1 {
		t.Fatalf("ожидался ровно один скроббл, получено %d: %+v", len(sink.scrobbles), sink.scrobbles)
	}
	if sink.scrobbles[0].Title != "Второй" {
		t.Errorf("засчитан неактивный плеер: %+v", sink.scrobbles[0].Track)
	}
}

func TestПлеерЗакрылся(t *testing.T) {
	s := newSim(t, DefaultOptions(), "vlc.exe")
	s.play(track("A", "B", 4*time.Minute))
	s.advance(30 * time.Second)

	s.engine.Update(media.State{ID: s.id, Name: s.name, Gone: true, At: s.now})
	if len(s.engine.Snapshots()) != 0 {
		t.Error("состояние закрытого плеера должно удаляться")
	}
	s.tick(5 * time.Minute)
	if len(s.sink.scrobbles) != 0 {
		t.Error("после закрытия плеера скробблов быть не должно")
	}
}

func TestОстановкаСбрасываетАктивныйПлеер(t *testing.T) {
	s := newSim(t, DefaultOptions(), "vlc.exe")
	s.play(track("A", "B", 4*time.Minute))
	s.advance(30 * time.Second)
	s.stop()
	s.advance(10 * time.Minute)
	if len(s.sink.scrobbles) != 0 {
		t.Error("после остановки трек не должен засчитываться")
	}
}

func TestБольшойРазрывВоВремени(t *testing.T) {
	sink := &fakeSink{}
	engine := New(DefaultOptions(), sink, quietLogger())
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	tr := track("A", "B", 10*time.Minute)
	engine.Update(media.State{ID: "bus", Name: "p", Status: media.Playing, Track: tr, At: now})

	now = now.Add(time.Hour)
	engine.Tick(now)

	if len(sink.scrobbles) != 0 {
		t.Errorf("сон системы не должен засчитываться как прослушивание: %+v", sink.scrobbles)
	}
}

func TestSnapshots(t *testing.T) {
	s := newSim(t, DefaultOptions(), "vlc.exe")
	s.play(track("A", "B", 4*time.Minute))
	s.advance(30 * time.Second)

	snaps := s.engine.Snapshots()
	if len(snaps) != 1 {
		t.Fatalf("ожидался один плеер, получено %d", len(snaps))
	}
	snap := snaps[0]
	if !snap.Active || snap.Status != media.Playing {
		t.Errorf("снимок = %+v", snap)
	}
	if snap.Target != 2*time.Minute {
		t.Errorf("порог = %s, ожидалось 2m", snap.Target)
	}
	if snap.Played < 29*time.Second || snap.Played > 31*time.Second {
		t.Errorf("прослушано = %s, ожидалось около 30s", snap.Played)
	}
}

func TestНачальнаяПозицияИгнорируется(t *testing.T) {
	opts := DefaultOptions()
	opts.IgnoreInitialPosition = true
	s := newSim(t, opts, "Spotify.exe")
	start := s.now
	s.track = track("A", "Новый трек", 4*time.Minute)
	s.pos = 3*time.Minute + 50*time.Second
	s.status = media.Playing
	s.engine.Update(s.state())

	s.tick(10 * time.Second)
	if len(s.sink.scrobbles) != 0 {
		t.Fatalf("устаревшая позиция засчитана как прослушивание: %+v", s.sink.scrobbles)
	}

	s.tick(111 * time.Second)
	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("трек должен засчитаться после половины реального прослушивания, получено %d", len(s.sink.scrobbles))
	}
	if got := s.sink.scrobbles[0].Timestamp; !got.Equal(start.UTC()) {
		t.Errorf("время начала = %s, ожидалось %s — момент, когда трек замечен", got, start.UTC())
	}
}

func (s *sim) played() time.Duration {
	s.t.Helper()
	snaps := s.engine.Snapshots()
	if len(snaps) != 1 {
		s.t.Fatalf("ожидался один плеер, получено %d", len(snaps))
	}
	return snaps[0].Played
}

func TestСвежейПозицииВериться(t *testing.T) {
	opts := DefaultOptions()
	opts.IgnoreInitialPosition = true
	s := newSim(t, opts, "Spotify.exe")
	s.track = track("100 gecs", "ringtone (remix)", 215*time.Second)
	s.pos = 3 * time.Second
	s.status = media.Playing
	st := s.state()
	st.PositionFresh = true
	s.engine.Update(st)

	if got := s.played(); got != 3*time.Second {
		t.Errorf("прослушано %s, ожидалось 3s — как у плеера", got)
	}
	s.tick(105 * time.Second)
	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("трек должен засчитаться к половине (107,5 с по плееру), получено %d", len(s.sink.scrobbles))
	}
	if want := s.now.Add(-108 * time.Second).UTC(); !s.sink.scrobbles[0].Timestamp.Equal(want) {
		t.Errorf("время начала = %s, ожидалось %s", s.sink.scrobbles[0].Timestamp, want)
	}
}

func TestСчётчикДогоняетСвежуюПозицию(t *testing.T) {
	opts := DefaultOptions()
	opts.IgnoreInitialPosition = true
	s := newSim(t, opts, "Spotify.exe")
	s.track = track("Charli XCX", "360", 133*time.Second)
	s.status = media.Playing
	s.pos = 4 * time.Second
	s.engine.Update(s.state())
	if got := s.played(); got != 0 {
		t.Fatalf("непроверенной позиции верить нельзя, прослушано %s", got)
	}

	s.now = s.now.Add(2 * time.Second)
	s.pos += 2 * time.Second
	st := s.state()
	st.PositionFresh = true
	s.engine.Update(st)
	if got := s.played(); got != 6*time.Second {
		t.Errorf("после подтверждения прослушано %s, ожидалось 6s", got)
	}

	s.now = s.now.Add(time.Second)
	s.pos += time.Minute
	st = s.state()
	st.PositionFresh = true
	s.engine.Update(st)
	if got := s.played(); got != 7*time.Second {
		t.Errorf("перемотка не должна засчитываться: прослушано %s, ожидалось 7s", got)
	}
}

func live(artist, title string) media.Track {
	return media.Track{Title: title, Artists: []string{artist}, Live: true}
}

func (s *sim) switchLive(tr media.Track, fromStart bool) {
	s.track = tr
	s.pos = 0
	s.status = media.Playing
	st := s.state()
	st.FromStart = fromStart
	s.engine.Update(st)
}

func TestПесняСРадиоЗасчитываетсяВКонце(t *testing.T) {
	s := newSim(t, DefaultOptions(), "radio")
	s.switchLive(live("Кино", "Кукушка"), false)
	s.advance(time.Minute)
	s.switchLive(live("Lana Del Rey", "Summertime Sadness"), true)
	if len(s.sink.scrobbles) != 0 {
		t.Fatal("песня, начало которой пропущено, засчитываться не должна")
	}
	start := s.now
	s.advance(3*time.Minute + 20*time.Second)
	if len(s.sink.scrobbles) != 0 {
		t.Fatal("песня с радио не должна засчитываться, пока звучит")
	}
	s.switchLive(live("MGK", "bloody valentine"), true)
	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("дослушанная песня должна засчитаться при смене, получено %d", len(s.sink.scrobbles))
	}
	sc := s.sink.scrobbles[0]
	if sc.Title != "Summertime Sadness" || !sc.Radio || !sc.Timestamp.Equal(start) {
		t.Errorf("скроббл: %+v", sc)
	}
	if sc.Duration != 3*time.Minute+20*time.Second {
		t.Errorf("длина песни должна равняться прослушанному: %s", sc.Duration)
	}
}

func TestКороткаяВставкаНаРадиоНеЗасчитывается(t *testing.T) {
	s := newSim(t, DefaultOptions(), "radio")
	s.switchLive(live("Радио", "Джингл"), true)
	s.advance(15 * time.Second)
	s.switchLive(live("Charli XCX", "360"), true)
	if len(s.sink.scrobbles) != 0 {
		t.Error("вставка короче 30 секунд засчитываться не должна")
	}
}

func TestОстановленнуюПеснюСРадиоНеЗасчитываем(t *testing.T) {
	s := newSim(t, DefaultOptions(), "radio")
	s.switchLive(live("Кино", "Кукушка"), true)
	s.advance(2 * time.Minute)
	s.stop()
	s.switchLive(live("Кино", "Группа крови"), true)
	if len(s.sink.scrobbles) != 0 {
		t.Error("песня, которую не дослушали до конца, засчитываться не должна")
	}
}

func TestДлиннаяПесняСРадиоЗасчитываетсяЧерезЧетыреМинуты(t *testing.T) {
	s := newSim(t, DefaultOptions(), "radio")
	s.switchLive(live("Кино", "Кукушка"), false)
	s.advance(4*time.Minute + 5*time.Second)
	if len(s.sink.scrobbles) != 1 || !s.sink.scrobbles[0].Radio {
		t.Fatalf("через 4 минуты песня с радио засчитывается и без начала: %+v", s.sink.scrobbles)
	}
	s.switchLive(live("Кино", "Группа крови"), true)
	if len(s.sink.scrobbles) != 1 {
		t.Error("уже засчитанная песня не засчитывается второй раз")
	}
}

func (s *sim) snap() Snapshot {
	s.t.Helper()
	snaps := s.engine.SnapshotsAt(s.now)
	if len(snaps) != 1 {
		s.t.Fatalf("ожидался один плеер, получено %d", len(snaps))
	}
	return snaps[0]
}

func TestПеремоткаВпередЗасчитываетсяПоПозицииПлеера(t *testing.T) {
	s := newSim(t, DefaultOptions(), "Spotify.exe")
	s.knownPos = true
	s.play(track("Charli xcx", "Delicious", 4*time.Minute+33*time.Second))
	s.advance(11 * time.Second)

	s.seek(2*time.Minute + 28*time.Second)
	sn := s.snap()
	if !sn.Synced || sn.Position != 2*time.Minute+28*time.Second {
		t.Fatalf("окно должно показывать позицию плеера 2:28: %+v", sn)
	}
	if sn.Left != 19*time.Second {
		t.Errorf("до засчитывания %s, ожидалось 19s — добрать 30 секунд прослушивания", sn.Left)
	}
	if len(s.sink.scrobbles) != 0 {
		t.Fatal("после 11 секунд прослушивания трек засчитывать рано")
	}

	s.advance(18 * time.Second)
	if len(s.sink.scrobbles) != 0 {
		t.Fatal("засчитано раньше 30 секунд прослушивания")
	}
	s.advance(2 * time.Second)
	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("после перемотки за середину и 30 секунд прослушивания трек засчитывается, получено %d", len(s.sink.scrobbles))
	}
	if got := s.snap().Position; got < 2*time.Minute+48*time.Second || got > 2*time.Minute+50*time.Second {
		t.Errorf("позиция идёт вместе с плеером: %s", got)
	}
}

func TestПеремоткаНазадНеОткладываетСкроббл(t *testing.T) {
	s := newSim(t, DefaultOptions(), "Spotify.exe")
	s.knownPos = true
	s.play(track("Tyler, The Creator", "Ring Ring Ring", 4*time.Minute+32*time.Second))
	s.advance(2 * time.Minute)
	s.seek(30 * time.Second)
	if sn := s.snap(); sn.Position != 30*time.Second || sn.Left != 16*time.Second {
		t.Fatalf("после перемотки назад: позиция %s, до засчитывания %s", sn.Position, sn.Left)
	}
	s.advance(17 * time.Second)
	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("2:16 прослушивания достаточно и после перемотки назад, получено %d", len(s.sink.scrobbles))
	}
}

func TestПеремоткаВКонецБезПрослушиванияНеЗасчитывается(t *testing.T) {
	s := newSim(t, DefaultOptions(), "Spotify.exe")
	s.knownPos = true
	s.play(track("Kero Kero Bonito", "Only Acting", 3*time.Minute+30*time.Second))
	s.advance(5 * time.Second)
	s.seek(3*time.Minute + 20*time.Second)
	s.advance(10 * time.Second)
	if len(s.sink.scrobbles) != 0 {
		t.Fatalf("15 секунд прослушивания не засчитываются, сколько ни перематывай: %+v", s.sink.scrobbles)
	}
	s.play(track("Kero Kero Bonito", "Flamingo", 3*time.Minute))
	if len(s.sink.scrobbles) != 0 {
		t.Fatal("пропущенный перемоткой трек засчитан при смене трека")
	}
}

func TestПозицияИзWindowsОтПрошлогоТрекаНеВерится(t *testing.T) {
	opts := DefaultOptions()
	opts.IgnoreInitialPosition = true
	s := newSim(t, opts, "Spotify.exe")
	s.knownPos = true
	s.reportedAt = s.now
	s.play(track("A", "Первый", 3*time.Minute))
	s.advance(10 * time.Second)

	stale := s.now.Add(-20 * time.Second)
	s.reportedAt = stale
	s.track = track("A", "Второй", 4*time.Minute)
	s.pos = 2*time.Minute + 50*time.Second
	s.engine.Update(s.state())
	if sn := s.snap(); sn.Synced {
		t.Fatalf("позиция, сообщённая до смены трека, относится к прошлому треку: %+v", sn)
	}
	s.tick(40 * time.Second)
	if len(s.sink.scrobbles) != 0 {
		t.Fatal("устаревшая позиция не должна ускорять засчитывание")
	}

	s.reportedAt = s.now
	s.pos = 40 * time.Second
	s.engine.Update(s.state())
	if sn := s.snap(); !sn.Synced || sn.Position != 40*time.Second {
		t.Errorf("новое сообщение плеера о позиции принимается: %+v", sn)
	}
}

func TestПервыйТрекПослеЗапускаПоказываетсяСПозиции(t *testing.T) {
	opts := DefaultOptions()
	opts.IgnoreInitialPosition = true
	s := newSim(t, opts, "Spotify.exe")
	start := s.now
	s.knownPos = true
	s.reportedAt = s.now.Add(-90 * time.Second)
	s.track = track("Charli xcx", "Lightning", 4*time.Minute)
	s.pos = 90 * time.Second
	s.status = media.Playing
	s.engine.Update(s.state())

	sn := s.snap()
	if !sn.Synced || sn.Position != 90*time.Second || sn.Played != 0 {
		t.Fatalf("программа запущена посреди трека: %+v", sn)
	}
	s.tick(10 * time.Second)
	if got := s.snap().Position; got != 100*time.Second {
		t.Errorf("позиция идёт между сообщениями плеера: %s", got)
	}
	s.pos = 100 * time.Second
	s.advance(19 * time.Second)
	if len(s.sink.scrobbles) != 0 {
		t.Fatal("29 секунд прослушивания мало")
	}
	s.advance(2 * time.Second)
	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("трек за серединой засчитывается после 30 секунд прослушивания, получено %d", len(s.sink.scrobbles))
	}
	if want := start.Add(-90 * time.Second).UTC(); !s.sink.scrobbles[0].Timestamp.Equal(want) {
		t.Errorf("время начала %s, ожидалось %s", s.sink.scrobbles[0].Timestamp, want)
	}
}

func TestНаПаузеПозицияСтоит(t *testing.T) {
	s := newSim(t, DefaultOptions(), "aimp")
	s.knownPos = true
	s.play(track("A", "B", 4*time.Minute))
	s.advance(20 * time.Second)
	s.pause()
	s.tick(30 * time.Second)
	if got := s.snap().Position; got != 20*time.Second {
		t.Errorf("на паузе позиция %s, ожидалось 20s", got)
	}
}

func TestПослеПаузыПозицияИзWindowsНеУбегает(t *testing.T) {
	opts := DefaultOptions()
	opts.IgnoreInitialPosition = true
	s := newSim(t, opts, "Spotify.exe")
	s.knownPos = true
	s.reportedAt = s.now
	s.play(track("A", "B", 4*time.Minute))
	s.advance(20 * time.Second)
	s.pause()
	s.tick(time.Minute)

	s.pos = s.now.Sub(s.reportedAt)
	s.resume()
	s.tick(10 * time.Second)
	if got := s.snap().Position; got != 30*time.Second {
		t.Errorf("после паузы позиция %s, ожидалось 30s: старое сообщение плеера не знает о паузе", got)
	}
}

func TestЗастывшаяПозицияНеПоказывается(t *testing.T) {
	s := newSim(t, DefaultOptions(), "aimp")
	s.knownPos = true
	s.play(track("A", "B", 4*time.Minute))
	poll := func(d time.Duration) {
		for left := d; left > 0; left -= 2 * time.Second {
			s.now = s.now.Add(2 * time.Second)
			s.engine.Update(s.state())
		}
	}
	poll(6 * time.Second)
	if sn := s.snap(); sn.Synced || sn.Position != 6*time.Second {
		t.Fatalf("плеер всё время сообщает 0:00 — позиции не верим, считаем сами: %+v", sn)
	}
	poll(110 * time.Second)
	if len(s.sink.scrobbles) != 0 {
		t.Fatal("засчитано раньше половины трека")
	}
	poll(4 * time.Second)
	if len(s.sink.scrobbles) != 1 {
		t.Fatalf("трек засчитывается по своему счёту, получено %d", len(s.sink.scrobbles))
	}
}

func TestПозицияСноваПринимаетсяКогдаПлеерОжил(t *testing.T) {
	s := newSim(t, DefaultOptions(), "aimp")
	s.knownPos = true
	s.play(track("A", "B", 4*time.Minute))
	s.advance(20 * time.Second)
	for i := 0; i < 3; i++ {
		s.now = s.now.Add(2 * time.Second)
		s.engine.Update(s.state())
	}
	if s.snap().Synced {
		t.Fatal("позиция стоит на месте во время воспроизведения — не верим ей")
	}
	s.advance(5 * time.Second)
	if sn := s.snap(); !sn.Synced || sn.Position != 25*time.Second {
		t.Errorf("плеер снова сообщает живую позицию: %+v", sn)
	}
}

func TestСообщениеWindowsОтПрошлогоТрекаНеПереносится(t *testing.T) {
	opts := DefaultOptions()
	opts.IgnoreInitialPosition = true
	s := newSim(t, opts, "Spotify.exe")
	s.knownPos = true
	s.reportedAt = s.now
	s.play(track("A", "Первый", 3*time.Minute))
	s.advance(time.Minute)

	s.track = track("A", "Второй", 4*time.Minute)
	s.pos = s.now.Sub(s.reportedAt)
	s.engine.Update(s.state())
	if sn := s.snap(); sn.Synced {
		t.Fatalf("сообщение о позиции осталось от прошлого трека: %+v", sn)
	}

	s.reportedAt = s.now.Add(-1500 * time.Millisecond)
	s.pos = 1500 * time.Millisecond
	s.track = track("A", "Третий", 4*time.Minute)
	s.engine.Update(s.state())
	if sn := s.snap(); !sn.Synced || sn.Position != 1500*time.Millisecond {
		t.Errorf("плеер сообщил о новом треке чуть раньше, чем программа его увидела: %+v", sn)
	}
}
