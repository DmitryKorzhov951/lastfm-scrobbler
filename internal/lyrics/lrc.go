package lyrics

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type Line struct {
	At   float64 `json:"t"`
	Text string  `json:"text"`
}

type Lyrics struct {
	Plain        string `json:"plain,omitempty"`
	Synced       []Line `json:"synced,omitempty"`
	Instrumental bool   `json:"instrumental,omitempty"`
	Source       string `json:"source,omitempty"`
}

const (
	FromFile   = "file"
	FromTags   = "tags"
	FromFolder = "folder"
	FromLRCLIB = "lrclib"
	FromOVH    = "lyricsovh"
)

func (l Lyrics) Empty() bool {
	return !l.Instrumental && strings.TrimSpace(l.Plain) == "" && len(l.Synced) == 0
}

var (
	stampRe = regexp.MustCompile(`^\[(\d{1,3}):(\d{1,2}(?:[.:]\d{1,3})?)\]`)
	tagRe   = regexp.MustCompile(`^\[([A-Za-z#]+):(.*)\]$`)
	wordRe  = regexp.MustCompile(`<\d{1,3}:\d{1,2}(?:[.:]\d{1,3})?>`)
)

func ParseLRC(text string) []Line {
	var out []Line
	offset := 0.0
	for _, raw := range strings.Split(newlines(text), "\n") {
		line := strings.TrimSpace(raw)
		var stamps []float64
		for {
			m := stampRe.FindStringSubmatch(line)
			if m == nil {
				break
			}
			min, _ := strconv.Atoi(m[1])
			sec, err := strconv.ParseFloat(strings.Replace(m[2], ":", ".", 1), 64)
			if err != nil || sec >= 60 {
				break
			}
			stamps = append(stamps, float64(min)*60+sec)
			line = strings.TrimSpace(line[len(m[0]):])
		}
		if len(stamps) == 0 {
			if m := tagRe.FindStringSubmatch(line); m != nil && strings.EqualFold(m[1], "offset") {
				if ms, err := strconv.Atoi(strings.TrimSpace(m[2])); err == nil {
					offset = float64(ms) / 1000
				}
			}
			continue
		}
		body := strings.Join(strings.Fields(wordRe.ReplaceAllString(line, "")), " ")
		for _, at := range stamps {
			out = append(out, Line{At: at, Text: body})
		}
	}
	for i := range out {
		out[i].At = max(0, out[i].At-offset)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

func FromText(text, source string) Lyrics {
	text = strings.TrimSpace(newlines(text))
	if text == "" {
		return Lyrics{}
	}
	if synced := ParseLRC(text); len(synced) >= 2 {
		return Lyrics{Synced: synced, Plain: plainOf(synced), Source: source}
	}
	return Lyrics{Plain: text, Source: source}
}

func plainOf(lines []Line) string {
	var b strings.Builder
	blank := true
	for _, l := range lines {
		if l.Text == "" {
			if !blank {
				b.WriteString("\n")
				blank = true
			}
			continue
		}
		b.WriteString(l.Text)
		b.WriteString("\n")
		blank = false
	}
	return strings.TrimSpace(b.String())
}

func newlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

func decode(data []byte) string {
	switch {
	case len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF:
		data = data[3:]
	case len(data) >= 2 && (data[0] == 0xFF && data[1] == 0xFE || data[0] == 0xFE && data[1] == 0xFF):
		big := data[0] == 0xFE
		units := make([]uint16, 0, len(data)/2)
		for i := 2; i+1 < len(data); i += 2 {
			if big {
				units = append(units, uint16(data[i])<<8|uint16(data[i+1]))
			} else {
				units = append(units, uint16(data[i+1])<<8|uint16(data[i]))
			}
		}
		return string(utf16.Decode(units))
	}
	if utf8.Valid(data) {
		return string(data)
	}
	var b strings.Builder
	b.Grow(len(data) * 2)
	for _, c := range data {
		b.WriteRune(cp1251(c))
	}
	return b.String()
}

var cp1251High = map[byte]rune{
	0x85: '…', 0x91: '‘', 0x92: '’', 0x93: '“', 0x94: '”', 0x96: '–', 0x97: '—',
	0xA0: ' ', 0xA8: 'Ё', 0xAB: '«', 0xB8: 'ё', 0xB9: '№', 0xBB: '»',
}

func cp1251(c byte) rune {
	switch {
	case c < 0x80:
		return rune(c)
	case c >= 0xC0:
		return rune(0x410 + int(c) - 0xC0)
	}
	if r, ok := cp1251High[c]; ok {
		return r
	}
	return '?'
}
