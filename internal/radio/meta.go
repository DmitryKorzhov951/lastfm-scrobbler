package radio

import (
	"bytes"
	"io"
	"strings"
	"unicode/utf8"
)

type metaReader struct {
	r        io.Reader
	interval int
	left     int
	onTitle  func(string)
	last     string
	seen     bool
}

func newMetaReader(r io.Reader, interval int, onTitle func(string)) *metaReader {
	return &metaReader{r: r, interval: interval, left: interval, onTitle: onTitle}
}

func (m *metaReader) Read(p []byte) (int, error) {
	if m.left == 0 {
		if err := m.readMeta(); err != nil {
			return 0, err
		}
		m.left = m.interval
	}
	if len(p) > m.left {
		p = p[:m.left]
	}
	n, err := m.r.Read(p)
	m.left -= n
	return n, err
}

func (m *metaReader) readMeta() error {
	var size [1]byte
	if _, err := io.ReadFull(m.r, size[:]); err != nil {
		return err
	}
	if size[0] == 0 {
		return nil
	}
	buf := make([]byte, int(size[0])*16)
	if _, err := io.ReadFull(m.r, buf); err != nil {
		return err
	}
	title, ok := parseStreamTitle(buf)
	if ok && (!m.seen || title != m.last) {
		m.seen, m.last = true, title
		m.onTitle(title)
	}
	return nil
}

func parseStreamTitle(meta []byte) (string, bool) {
	meta = bytes.TrimRight(meta, "\x00")
	s := decodeText(meta)
	const key = "StreamTitle='"
	i := strings.Index(s, key)
	if i < 0 {
		return "", false
	}
	s = s[i+len(key):]
	if j := strings.Index(s, "';Stream"); j >= 0 {
		s = s[:j]
	} else if j := strings.LastIndex(s, "';"); j >= 0 {
		s = s[:j]
	} else {
		s = strings.TrimSuffix(s, "'")
	}
	return strings.TrimSpace(s), true
}

func decodeText(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	table := &cp1252
	run := 0
	for _, c := range b {
		if c >= 0xC0 || c == 0xA8 || c == 0xB8 {
			run++
			if run >= 3 {
				table = &cp1251
				break
			}
		} else {
			run = 0
		}
	}
	var sb strings.Builder
	sb.Grow(len(b) * 2)
	for _, c := range b {
		if c < 0x80 {
			sb.WriteByte(c)
		} else {
			sb.WriteRune(table[c-0x80])
		}
	}
	return sb.String()
}

func SplitTitle(s string) (artist, title string, ok bool) {
	s = strings.TrimSpace(s)
	best := -1
	sepLen := 0
	for _, sep := range []string{" - ", " – ", " — "} {
		if i := strings.Index(s, sep); i > 0 && (best < 0 || i < best) {
			best, sepLen = i, len(sep)
		}
	}
	if best < 0 {
		return "", s, false
	}
	artist = strings.TrimSpace(s[:best])
	title = strings.TrimSpace(s[best+sepLen:])
	if artist == "" || title == "" {
		return "", s, false
	}
	return artist, title, true
}
