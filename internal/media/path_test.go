package media

import (
	"path/filepath"
	"testing"
)

func TestПутьИзАдресаФайла(t *testing.T) {
	cases := map[string]string{
		"file:///home/u/Музыка/cover.jpg":        "/home/u/Музыка/cover.jpg",
		"file:///home/u/a%20b/c.png":             "/home/u/a b/c.png",
		"/home/u/x.mp3":                          "/home/u/x.mp3",
		"https://i.scdn.co/image/ab67616d0000b2": "",
		"http://radio.example/stream":            "",
		"relative/path.mp3":                      "",
	}
	for in, want := range cases {
		if got := filepath.ToSlash(LocalPath(in)); got != want {
			t.Errorf("LocalPath(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}
