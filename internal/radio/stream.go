package radio

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ErrHLS = errors.New("станция вещает в формате HLS (.m3u8) — такой поток пока не поддерживается. " +
	"Поищите на сайте станции адрес потока MP3 или AAC")

var errWebPage = errors.New("по адресу открывается веб-страница, а не поток. " +
	"Нужен прямой адрес потока: его дают ссылкой для плеера или файлом .m3u / .pls")

type Stream struct {
	audio       io.Reader
	body        io.Closer
	ContentType string
	Codec       string
	Bitrate     int
	Name        string
}

func (s *Stream) Read(p []byte) (int, error) { return s.audio.Read(p) }

func (s *Stream) Close() error { return s.body.Close() }

func (s *Stream) Format() string {
	codec := s.Codec
	if codec == "" {
		codec = "поток"
	}
	if s.Bitrate > 0 {
		return fmt.Sprintf("%s · %d кбит/с", codec, s.Bitrate)
	}
	return codec
}

func NewClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				c, err := dialer.DialContext(ctx, network, addr)
				if err != nil {
					return nil, err
				}
				return &icyConn{Conn: c}, nil
			},
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			DisableCompression:    true,
			MaxIdleConns:          4,
			IdleConnTimeout:       30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("слишком много перенаправлений")
			}
			return nil
		},
	}
}

type icyConn struct {
	net.Conn
	started bool
	pending []byte
	err     error
}

func (c *icyConn) Read(p []byte) (int, error) {
	if !c.started {
		c.started = true
		buf := make([]byte, 0, 512)
		tmp := make([]byte, 512)
		for len(buf) < 4 {
			n, err := c.Conn.Read(tmp)
			buf = append(buf, tmp[:n]...)
			if err != nil {
				c.err = err
				break
			}
		}
		if bytes.HasPrefix(buf, []byte("ICY ")) {
			buf = append([]byte("HTTP/1.0 "), buf[4:]...)
		}
		c.pending = buf
	}
	if len(c.pending) > 0 {
		n := copy(p, c.pending)
		c.pending = c.pending[n:]
		return n, nil
	}
	if c.err != nil {
		err := c.err
		c.err = nil
		return 0, err
	}
	return c.Conn.Read(p)
}

const maxHops = 5

func Connect(ctx context.Context, client *http.Client, rawURL, userAgent string, onTitle func(string)) (*Stream, error) {
	u := rawURL
	for hop := 0; hop < maxHops; hop++ {
		resp, err := get(ctx, client, u, userAgent)
		if err != nil {
			return nil, err
		}
		ct := mediaType(resp.Header.Get("Content-Type"))
		br := bufio.NewReaderSize(resp.Body, 16<<10)
		head, _ := br.Peek(16)

		if isPlaylist(ct, resp.Request.URL.Path, head) {
			data, err := io.ReadAll(io.LimitReader(br, 64<<10))
			resp.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("не удалось прочитать плейлист: %w", err)
			}
			next, err := parsePlaylist(decodeText(data), resp.Request.URL)
			if err != nil {
				return nil, err
			}
			u = next
			continue
		}

		codec, outType, err := detect(ct, head)
		if err != nil {
			resp.Body.Close()
			return nil, err
		}
		s := &Stream{
			audio:       br,
			body:        resp.Body,
			ContentType: outType,
			Codec:       codec,
			Bitrate:     headerInt(resp.Header.Get("icy-br")),
			Name:        strings.TrimSpace(decodeText([]byte(resp.Header.Get("icy-name")))),
		}
		if n := headerInt(resp.Header.Get("icy-metaint")); n > 0 && n <= 1<<20 && onTitle != nil {
			s.audio = newMetaReader(br, n, onTitle)
		}
		return s, nil
	}
	return nil, errors.New("плейлисты ссылаются друг на друга слишком долго — нужен прямой адрес потока")
}

func get(ctx context.Context, client *http.Client, u, userAgent string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("неверный адрес станции: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Icy-MetaData", "1")
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("не удалось подключиться к станции: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("станция ответила ошибкой: %s", resp.Status)
	}
	return resp, nil
}

func mediaType(ct string) string {
	t, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(ct))
	}
	return t
}

func headerInt(v string) int {
	v, _, _ = strings.Cut(strings.TrimSpace(v), ",")
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

var playlistTypes = map[string]bool{
	"audio/x-scpls":                 true,
	"audio/scpls":                   true,
	"application/pls+xml":           true,
	"audio/x-mpegurl":               true,
	"audio/mpegurl":                 true,
	"application/x-mpegurl":         true,
	"application/mpegurl":           true,
	"application/vnd.apple.mpegurl": true,
}

func isPlaylist(ct, urlPath string, head []byte) bool {
	if playlistTypes[ct] {
		return true
	}
	textual := ct == "" || strings.HasPrefix(ct, "text/plain") || ct == "application/octet-stream"
	if !textual {
		return false
	}
	switch strings.ToLower(path.Ext(urlPath)) {
	case ".pls", ".m3u", ".m3u8":
		return true
	}
	h := strings.ToLower(string(bytes.TrimLeft(head, "\uFEFF \r\n\t")))
	for _, p := range []string{"[playlist]", "#extm3u", "http://", "https://"} {
		if strings.HasPrefix(h, p) {
			return true
		}
	}
	return false
}

var plsKey = regexp.MustCompile(`^([A-Za-z]+)(\d*)=(.*)$`)

func parsePlaylist(text string, base *url.URL) (string, error) {
	if strings.Contains(text, "#EXT-X-") {
		return "", ErrHLS
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\uFEFF"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		if m := plsKey.FindStringSubmatch(line); m != nil {
			if !strings.EqualFold(m[1], "file") {
				continue
			}
			line = strings.TrimSpace(m[3])
		}
		ref, err := base.Parse(line)
		if err != nil || (ref.Scheme != "http" && ref.Scheme != "https") {
			continue
		}
		return ref.String(), nil
	}
	return "", errors.New("в плейлисте нет адреса потока")
}

func detect(ct string, head []byte) (codec, contentType string, err error) {
	switch ct {
	case "audio/mpeg", "audio/mp3", "audio/x-mpeg", "audio/mpeg3", "audio/x-mp3", "audio/mpg":
		return "MP3", "audio/mpeg", nil
	case "audio/aac", "audio/aacp", "audio/x-aac", "audio/x-aacp":
		return "AAC", "audio/aac", nil
	case "audio/ogg", "application/ogg", "audio/x-ogg", "audio/vorbis", "audio/opus":
		return "Ogg", "audio/ogg", nil
	case "audio/flac", "audio/x-flac":
		return "FLAC", "audio/flac", nil
	case "audio/webm":
		return "WebM", "audio/webm", nil
	case "audio/wav", "audio/x-wav", "audio/wave":
		return "WAV", "audio/wav", nil
	}
	switch {
	case bytes.HasPrefix(head, []byte("ID3")):
		return "MP3", "audio/mpeg", nil
	case bytes.HasPrefix(head, []byte("OggS")):
		return "Ogg", "audio/ogg", nil
	case bytes.HasPrefix(head, []byte("fLaC")):
		return "FLAC", "audio/flac", nil
	case len(head) >= 2 && head[0] == 0xFF && head[1]&0xE0 == 0xE0:
		if head[1]&0x06 == 0 {
			return "AAC", "audio/aac", nil
		}
		return "MP3", "audio/mpeg", nil
	}
	low := strings.ToLower(string(bytes.TrimSpace(head)))
	if strings.HasPrefix(ct, "text/html") || strings.HasPrefix(low, "<!doctype") || strings.HasPrefix(low, "<html") {
		return "", "", errWebPage
	}
	if strings.HasPrefix(ct, "video/") || strings.HasPrefix(ct, "image/") || strings.HasPrefix(ct, "text/") {
		return "", "", fmt.Errorf("по адресу не звук, а %s — нужен адрес аудиопотока", ct)
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	return "", ct, nil
}
