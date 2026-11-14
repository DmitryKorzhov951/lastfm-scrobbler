package gsmtc

import (
	"testing"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

func TestAppName(t *testing.T) {
	cases := map[string]string{
		"Spotify.exe":      "spotify",
		"Chrome":           "chrome",
		"MSEdge":           "msedge",
		"308046B0AF4A39CB": "firefox",
		"Microsoft.ZuneMusic_8wekyb3d8bbwe!Microsoft.ZuneMusic": "zunemusic",
		"SpotifyAB.SpotifyMusic_zpdnekdrzrea0!Spotify":          "spotify",
		`C:\Program Files\foobar2000\foobar2000.exe`:            "foobar2000",
		"AIMP.EXE":      "aimp",
		"my_player.exe": "my_player",
		"":              "unknown",
		"   ":           "unknown",
	}
	for in, want := range cases {
		if got := AppName(in); got != want {
			t.Errorf("AppName(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}

func TestIdentity(t *testing.T) {
	if got := Identity("zunemusic"); got != "Медиаплеер Windows" {
		t.Errorf("Identity(zunemusic) = %q", got)
	}
	if got := Identity("неизвестный"); got != "неизвестный" {
		t.Errorf("для неизвестного плеера должно возвращаться само имя, получено %q", got)
	}
}

func TestMapStatus(t *testing.T) {
	cases := map[int32]media.Status{
		statusPlaying:  media.Playing,
		statusPaused:   media.Paused,
		statusChanging: media.Paused,
		statusStopped:  media.Stopped,
		statusOpened:   media.Stopped,
		statusClosed:   media.Stopped,
		42:             media.Unknown,
	}
	for in, want := range cases {
		if got := MapStatus(in); got != want {
			t.Errorf("MapStatus(%d) = %q, ожидалось %q", in, got, want)
		}
	}
}

func TestFromTimeSpan(t *testing.T) {
	if got := FromTimeSpan(2_000_000_000); got != 200*time.Second {
		t.Errorf("FromTimeSpan = %s, ожидалось 200s", got)
	}
}

func TestAllowed(t *testing.T) {
	if !Allowed("spotify", nil, nil) {
		t.Error("при пустых списках разрешено всё")
	}
	if Allowed("chrome", nil, []string{"Chrome"}) {
		t.Error("сравнение должно игнорировать регистр")
	}
	if Allowed("chrome", []string{"chrome"}, []string{"chrome"}) {
		t.Error("список игнорируемых важнее списка разрешённых")
	}
	if Allowed("aimp", []string{"spotify"}, nil) {
		t.Error("при непустом списке разрешённых остальные отсекаются")
	}
}

func TestToState(t *testing.T) {
	at := time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)
	st := ToState(Raw{
		AUMID:       "Spotify.exe",
		Status:      statusPlaying,
		Title:       "  Кукушка ",
		Artist:      "Кино",
		Album:       "Чёрный альбом",
		AlbumArtist: "Кино",
		TrackNumber: 3,
		Start:       0,
		End:         3_600_000_000,
		Position:    600_000_000,
	}, at)

	if st.ID != "Spotify.exe" || st.Name != "spotify" || st.Identity != "Spotify" {
		t.Errorf("идентификация плеера: %+v", st)
	}
	if st.Status != media.Playing {
		t.Errorf("Status = %q", st.Status)
	}
	if st.Track.Title != "Кукушка" || st.Track.Artist() != "Кино" || st.Track.Album != "Чёрный альбом" {
		t.Errorf("метаданные: %+v", st.Track)
	}
	if st.Track.Length != 6*time.Minute {
		t.Errorf("Length = %s", st.Track.Length)
	}
	if st.Position != time.Minute {
		t.Errorf("Position = %s", st.Position)
	}
	if st.Track.TrackNumber != 3 {
		t.Errorf("TrackNumber = %d", st.Track.TrackNumber)
	}
	if !st.At.Equal(at) || !st.Track.Valid() {
		t.Errorf("снимок неполный: %+v", st)
	}
}

func TestToStateИсполнительНеДелится(t *testing.T) {
	st := ToState(Raw{AUMID: "Spotify.exe", Title: "EARFQUAKE", Artist: "Tyler, The Creator"}, time.Now())
	if st.Track.Artist() != "Tyler, The Creator" {
		t.Errorf("исполнитель с запятой в имени разрезан: %q", st.Track.Artist())
	}
}

func TestToStateПозицияВПределахТрека(t *testing.T) {
	over := ToState(Raw{AUMID: "x", Start: 0, End: 1_000_000_000, Position: 5_000_000_000}, time.Now())
	if over.Position != over.Track.Length {
		t.Errorf("позиция за концом трека должна обрезаться до длительности, получено %s", over.Position)
	}
	neg := ToState(Raw{AUMID: "x", Start: 1_000_000_000, End: 2_000_000_000, Position: 0}, time.Now())
	if neg.Position != 0 {
		t.Errorf("позиция до начала трека должна давать ноль, получено %s", neg.Position)
	}
}

func TestToStateБезДлительности(t *testing.T) {
	st := ToState(Raw{AUMID: "Chrome", Title: "Песня", Artist: "Радио", Position: 30_000_000_000}, time.Now())
	if st.Track.Length != 0 {
		t.Errorf("Length = %s, ожидался ноль", st.Track.Length)
	}
	if st.Position != 50*time.Minute {
		t.Errorf("без длительности позиция не обрезается, получено %s", st.Position)
	}
}

func TestToStateПустыеПоля(t *testing.T) {
	st := ToState(Raw{AUMID: "Chrome"}, time.Now())
	if st.Track.Valid() {
		t.Error("без исполнителя и названия трек не должен считаться пригодным")
	}
	if len(st.Track.Artists) != 0 || len(st.Track.AlbumArtists) != 0 {
		t.Errorf("пустые строки не должны превращаться в исполнителей: %+v", st.Track)
	}
}

func TestPickВкладкиБраузера(t *testing.T) {
	raws := []Raw{
		{AUMID: "Chrome", Status: statusPaused, Title: "Видео на YouTube"},
		{AUMID: "Spotify.exe", Status: statusPaused, Title: "Трек в Spotify"},
		{AUMID: "Chrome", Status: statusPlaying, Title: "Музыка во второй вкладке"},
		{AUMID: "chrome", Status: statusStopped, Title: "Третья вкладка"},
	}
	got := Pick(raws)
	if len(got) != 2 {
		t.Fatalf("ожидалось по одной сессии на приложение (2), получено %d: %+v", len(got), got)
	}
	if got[0].AUMID != "Chrome" || got[0].Title != "Музыка во второй вкладке" {
		t.Errorf("от браузера должна остаться играющая вкладка, получено %+v", got[0])
	}
	if got[1].Title != "Трек в Spotify" {
		t.Errorf("другие приложения не должны пострадать, получено %+v", got[1])
	}
}

func TestPickПорядокПриРавенстве(t *testing.T) {
	got := Pick([]Raw{
		{AUMID: "Chrome", Status: statusPlaying, Title: "первая"},
		{AUMID: "Chrome", Status: statusPlaying, Title: "вторая"},
	})
	if len(got) != 1 || got[0].Title != "первая" {
		t.Errorf("при равенстве должна оставаться первая сессия, получено %+v", got)
	}
}

func TestPickПустой(t *testing.T) {
	if got := Pick(nil); len(got) != 0 {
		t.Errorf("Pick(nil) = %+v", got)
	}
}

func winTime(t time.Time) int64 { return windowsToUnix + t.UnixNano()/100 }

func TestFromDateTime(t *testing.T) {
	want := time.Date(2026, 10, 18, 21, 30, 15, 500_000_000, time.UTC)
	if got := FromDateTime(winTime(want)); !got.Equal(want) {
		t.Errorf("FromDateTime = %s, ожидалось %s", got, want)
	}
	if !FromDateTime(0).IsZero() || !FromDateTime(-5).IsZero() {
		t.Error("нулевое и отрицательное значение должны давать нулевое время")
	}
}

func TestToStateСвежаяПозиция(t *testing.T) {
	at := time.Date(2026, 10, 18, 21, 30, 0, 0, time.UTC)
	r := Raw{
		AUMID: "Spotify.exe", Status: statusPlaying, Title: "ringtone (remix)", Artist: "100 gecs",
		End: 215 * 10_000_000, Position: 1 * 10_000_000, Updated: winTime(at.Add(-2 * time.Second)),
	}
	st := ToState(r, at)
	if !st.PositionFresh {
		t.Fatal("позиция, обновлённая 2 секунды назад, свежая")
	}
	if st.Position != 3*time.Second {
		t.Errorf("позиция = %s, ожидалось 3s", st.Position)
	}

	r.Updated = winTime(at.Add(-40 * time.Second))
	if st := ToState(r, at); st.PositionFresh || st.Position != 41*time.Second || !st.PositionKnown ||
		!st.PositionReported.Equal(at.Add(-40*time.Second)) {
		t.Errorf("давно обновлённая позиция не свежая, но идёт вместе с треком: %+v", st)
	}

	r.Updated = winTime(at.Add(-time.Second))
	r.Status = statusPaused
	if st := ToState(r, at); st.PositionFresh || st.Position != time.Second {
		t.Errorf("на паузе позиция не растёт: %+v", st)
	}

	r.Status, r.Updated = statusPlaying, 0
	if st := ToState(r, at); st.PositionFresh || st.PositionKnown {
		t.Error("без времени обновления позиция неизвестна")
	}

	r.Position, r.Updated = 214*10_000_000, winTime(at.Add(-5*time.Second))
	if st := ToState(r, at); st.Position != 215*time.Second {
		t.Errorf("позиция = %s, ожидалось не больше длины трека", st.Position)
	}
}
