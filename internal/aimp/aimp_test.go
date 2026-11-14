package aimp

import (
	"encoding/binary"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

func remoteInfo(header int, ptr uint64, durationMs, trackNum uint32, strs [6]string) []byte {
	buf := make([]byte, MapSize)
	le := binary.LittleEndian
	le.PutUint32(buf[offSize:], uint32(header))
	le.PutUint32(buf[offActive:], 1)
	le.PutUint32(buf[offDuration:], durationMs)
	le.PutUint32(buf[offTrackNum:], trackNum)
	for p := offLengths + 24; p+8 <= header; p += 8 {
		le.PutUint64(buf[p:], ptr)
	}
	pos := header
	for i, s := range strs {
		u := utf16.Encode([]rune(s))
		le.PutUint32(buf[offLengths+4*i:], uint32(len(u)))
		for _, c := range u {
			le.PutUint16(buf[pos:], c)
			pos += 2
		}
	}
	return buf
}

var track = [6]string{"Tree of Clues", "Ninajirachi", "2025", `C:\Music\Ninajirachi - All I Am.flac`, "Electronic", "All I Am"}

func TestParse32(t *testing.T) {
	info, err := Parse(remoteInfo(headerSize32, 0x00A1B2C3, 197_000, 7, track))
	if err != nil {
		t.Fatal(err)
	}
	if info.Artist != "Ninajirachi" || info.Title != "All I Am" || info.Album != "Tree of Clues" ||
		info.FileName != track[3] || info.Genre != "Electronic" || info.Date != "2025" {
		t.Errorf("строки разобраны неверно: %+v", info)
	}
	if info.Duration != 197*time.Second || info.TrackNumber != 7 || !info.Active {
		t.Errorf("числа разобраны неверно: %+v", info)
	}
}

func TestParse64(t *testing.T) {
	info, err := Parse(remoteInfo(headerSize64, 0x000001F4_5A6B7C80, 133_000, 1,
		[6]string{"BRAT", "Charli XCX", "", "", "", "360"}))
	if err != nil {
		t.Fatal(err)
	}
	if info.Artist != "Charli XCX" || info.Title != "360" || info.Album != "BRAT" {
		t.Errorf("64-битный заголовок разобран неверно: %+v", info)
	}
}

func TestParseКириллица(t *testing.T) {
	info, err := Parse(remoteInfo(headerSize32, 0x00A1B2C3, 336_000, 0,
		[6]string{"Звезда по имени Солнце", "Кино", "", "", "", "Кукушка"}))
	if err != nil {
		t.Fatal(err)
	}
	if info.Artist != "Кино" || info.Title != "Кукушка" {
		t.Errorf("кириллица: %+v", info)
	}
}

func TestParseИспорченныеДанные(t *testing.T) {
	if _, err := Parse(make([]byte, 10)); err == nil {
		t.Error("короткие данные должны отвергаться")
	}
	buf := remoteInfo(headerSize32, 0, 0, 0, track)
	binary.LittleEndian.PutUint32(buf[offLengths:], 5000)
	if _, err := Parse(buf); err == nil {
		t.Error("длина строки больше общей памяти должна отвергаться")
	}
	buf = remoteInfo(headerSize32, 0, 0, 0, track)
	binary.LittleEndian.PutUint32(buf[offLengths:], 1000)
	if _, err := Parse(buf); err == nil {
		t.Error("строки за пределами общей памяти должны отвергаться")
	}
}

func TestToState(t *testing.T) {
	at := time.Date(2026, 10, 18, 22, 0, 0, 0, time.UTC)
	info, _ := Parse(remoteInfo(headerSize32, 0, 197_000, 7, track))

	st := ToState(info, statePlaying, 64*time.Second, at)
	if st.Name != "aimp" || st.Identity != "AIMP" || st.Status != media.Playing {
		t.Errorf("плеер: %+v", st)
	}
	if st.Track.Artist() != "Ninajirachi" || st.Track.Title != "All I Am" || st.Track.Length != 197*time.Second {
		t.Errorf("трек: %+v", st.Track)
	}
	if st.Position != 64*time.Second || !st.PositionFresh {
		t.Errorf("позиция AIMP живая и свежая: %s, %v", st.Position, st.PositionFresh)
	}

	if st := ToState(info, statePaused, 64*time.Second, at); st.Status != media.Paused || st.PositionFresh {
		t.Errorf("пауза: %+v", st)
	}
	if st := ToState(info, stateStopped, 0, at); st.Status != media.Stopped {
		t.Errorf("стоп: %+v", st)
	}
}

func TestToStateБезТегов(t *testing.T) {
	info := Info{Title: "Lana Del Rey - Young and Beautiful", Duration: 236 * time.Second}
	st := ToState(info, statePlaying, 0, time.Now())
	if st.Track.Artist() != "Lana Del Rey" || st.Track.Title != "Young and Beautiful" {
		t.Errorf("исполнитель из имени файла: %q — %q", st.Track.Artist(), st.Track.Title)
	}
	info = Info{Artist: "MGK", Title: "my ex's best friend - remix"}
	if st := ToState(info, statePlaying, 0, time.Now()); st.Track.Title != "my ex's best friend - remix" {
		t.Errorf("название с дефисом испорчено: %q", st.Track.Title)
	}
}

func TestПозицияAIMPСМиллисекундами(t *testing.T) {
	cases := []struct {
		secs, ms uint64
		msOK     bool
		want     time.Duration
	}{
		{277, 277_940, true, 277940 * time.Millisecond},
		{277, 0, false, 277 * time.Second},
		{277, 0, true, 277 * time.Second},
		{277, 4_000_000, true, 277 * time.Second},
		{0, 0, true, 0},
		{12, 11_990, true, 11990 * time.Millisecond},
	}
	for _, c := range cases {
		if got := Position(c.secs, c.ms, c.msOK); got != c.want {
			t.Errorf("Position(%d, %d, %t) = %s, ожидалось %s", c.secs, c.ms, c.msOK, got, c.want)
		}
	}
}
