package cover

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

func pngBytes(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func newStore(t *testing.T, dir string) *Store {
	t.Helper()
	s := New(dir, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(s.Wait)
	return s
}

func track(artist, title, album string) media.Track {
	return media.Track{Artists: []string{artist}, Title: title, Album: album}
}

func keyAfterLookup(s *Store, id string, tr media.Track) string {
	if k := s.Key(id, tr); k != "" {
		return k
	}
	s.Wait()
	return s.Key(id, tr)
}

func TestОбложкаОтПлеера(t *testing.T) {
	s := newStore(t, "")
	art := pngBytes(t, color.RGBA{200, 80, 180, 255})
	tr := track("Charli xcx", "360", "BRAT")
	s.Note(media.State{ID: "Spotify.exe", Track: tr, Art: art})

	k := s.Key("Spotify.exe", tr)
	if k == "" {
		t.Fatal("картинка от плеера показывается сразу")
	}
	img, ok := s.Image(k)
	if !ok || !bytes.Equal(img.Data, art) || img.Type != "image/png" {
		t.Fatalf("картинка: %v %q", ok, img.Type)
	}

	other := track("Charli xcx", "Apple", "BRAT")
	if k := s.Key("Spotify.exe", other); k != "" {
		t.Error("картинка прошлого трека не показывается у нового")
	}
}

func TestНеКартинкаОтПлеераНеПринимается(t *testing.T) {
	s := newStore(t, "")
	tr := track("A", "B", "")
	s.Note(media.State{ID: "p", Track: tr, Art: []byte("<html>это не картинка</html>")})
	if k := keyAfterLookup(s, "p", tr); k != "" {
		t.Error("не картинка не становится обложкой")
	}
}

func id3WithPicture(pic []byte, mime string) []byte {
	var frame bytes.Buffer
	frame.WriteByte(0)
	frame.WriteString(mime)
	frame.WriteByte(0)
	frame.WriteByte(3)
	frame.WriteByte(0)
	frame.Write(pic)

	var body bytes.Buffer
	body.WriteString("APIC")
	binary.Write(&body, binary.BigEndian, uint32(frame.Len()))
	body.Write([]byte{0, 0})
	body.Write(frame.Bytes())

	n := body.Len()
	var out bytes.Buffer
	out.WriteString("ID3")
	out.Write([]byte{3, 0, 0})
	out.Write([]byte{byte(n >> 21 & 0x7f), byte(n >> 14 & 0x7f), byte(n >> 7 & 0x7f), byte(n & 0x7f)})
	out.Write(body.Bytes())
	out.Write(bytes.Repeat([]byte{0xFF, 0xFB, 0x90, 0x00}, 64))
	return out.Bytes()
}

func TestОбложкаИзТеговMP3(t *testing.T) {
	dir := t.TempDir()
	art := jpegBytes(t)
	path := filepath.Join(dir, "01 - Depression Part I.mp3")
	if err := os.WriteFile(path, id3WithPicture(art, "image/jpeg"), 0o600); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "folder.png"), pngBytes(t, color.RGBA{9, 9, 9, 255}), 0o600)

	s := newStore(t, "")
	tr := track("sic Hate", "Depression: Part I", "")
	tr.URL = path
	s.Note(media.State{ID: "aimp", Track: tr})
	k := keyAfterLookup(s, "aimp", tr)
	img, ok := s.Image(k)
	if !ok || !bytes.Equal(img.Data, art) || img.Type != "image/jpeg" {
		t.Fatalf("обложка из тегов файла важнее картинки в папке: %v %q", ok, img.Type)
	}
}

func TestОбложкаИзПапкиАльбома(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "track.flac")
	os.WriteFile(path, []byte("fLaC не настоящий"), 0o600)
	front := pngBytes(t, color.RGBA{10, 20, 30, 255})
	os.WriteFile(filepath.Join(dir, "back.jpg"), jpegBytes(t), 0o600)
	os.WriteFile(filepath.Join(dir, "Front.PNG"), front, 0o600)

	if got := FromFile(path); !bytes.Equal(got, front) {
		t.Fatal("из картинок в папке берётся обложка, а не задник")
	}

	os.Remove(filepath.Join(dir, "Front.PNG"))
	if got := FromFile(path); got == nil {
		t.Error("единственная картинка в папке и есть обложка")
	}
	os.WriteFile(filepath.Join(dir, "booklet.jpg"), jpegBytes(t), 0o600)
	if got := FromFile(path); got != nil {
		t.Error("из нескольких картинок без понятного имени ничего не выбираем")
	}
}

func TestОбложкаИзLastfmСохраняетсяНаДиск(t *testing.T) {
	art := pngBytes(t, color.RGBA{50, 60, 70, 255})
	var downloads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		w.Write(art)
	}))
	defer srv.Close()

	dir := t.TempDir()
	s := newStore(t, dir)
	var asked []string
	s.SetFinder(func(ctx context.Context, artist, title, album string) (string, error) {
		asked = append(asked, artist+"|"+title+"|"+album)
		return srv.URL + "/300x300/a.png", nil
	})
	tr := track("Charli xcx", "360", "BRAT")
	k := keyAfterLookup(s, "Spotify.exe", tr)
	if img, ok := s.Image(k); !ok || !bytes.Equal(img.Data, art) {
		t.Fatalf("обложка из Last.fm: %q", k)
	}
	if len(asked) != 1 || asked[0] != "Charli xcx|360|BRAT" {
		t.Errorf("запросы: %v", asked)
	}

	again := track("charli xcx", "Apple", "brat")
	if k2 := keyAfterLookup(s, "Spotify.exe", again); k2 != k || len(asked) != 1 || downloads.Load() != 1 {
		t.Errorf("второй трек того же альбома берёт обложку с диска: %q %v %d", k2, asked, downloads.Load())
	}

	fresh := newStore(t, dir)
	fresh.SetFinder(func(context.Context, string, string, string) (string, error) {
		t.Error("после перезапуска обложка берётся с диска без Last.fm")
		return "", nil
	})
	if k3 := keyAfterLookup(fresh, "x", tr); k3 != k {
		t.Errorf("с диска: %q", k3)
	}
}

func TestНенайденнуюОбложкуНеИщемСнова(t *testing.T) {
	s := newStore(t, "")
	calls := 0
	s.SetFinder(func(context.Context, string, string, string) (string, error) {
		calls++
		return "", nil
	})
	tr := track("Никто", "Ничего", "")
	for i := 0; i < 3; i++ {
		if k := keyAfterLookup(s, "p", tr); k != "" {
			t.Fatal("обложки нет")
		}
	}
	if calls != 1 {
		t.Errorf("Last.fm спрошен %d раз, ожидался один", calls)
	}
}

func TestПослеОшибкиСетиОбложкаИщетсяПозже(t *testing.T) {
	s := newStore(t, "")
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	calls := 0
	s.SetFinder(func(context.Context, string, string, string) (string, error) {
		calls++
		return "", errors.New("нет связи")
	})
	tr := track("A", "B", "C")
	keyAfterLookup(s, "p", tr)
	keyAfterLookup(s, "p", tr)
	if calls != 1 {
		t.Fatalf("сразу после ошибки повторять рано: %d", calls)
	}
	now = now.Add(3 * time.Minute)
	keyAfterLookup(s, "p", tr)
	if calls != 2 {
		t.Errorf("через пару минут ищем снова: %d", calls)
	}
}
