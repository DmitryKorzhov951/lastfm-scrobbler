package ui

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"rsc.io/qr"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/library"
)

type PhoneBackend interface {
	State() State
	History(limit int) []Scrobble
	Top(kind, period string, offset int) (library.Top, error)
}

type PhoneInfo struct {
	Enabled bool     `json:"enabled"`
	URL     string   `json:"url,omitempty"`
	Other   []string `json:"other,omitempty"`
	QR      []string `json:"qr,omitempty"`
	Error   string   `json:"error,omitempty"`
}

type PhoneNow struct {
	User   string      `json:"user"`
	Now    *NowPlaying `json:"now"`
	Paused bool        `json:"paused"`
	Today  Totals      `json:"today"`
	Theme  string      `json:"theme"`
	Bursts bool        `json:"bursts"`
}

const (
	PhoneHistoryMax = 100
	phonePrefix     = "/p/"
)

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

type Phone struct {
	backend PhoneBackend
	token   string
	log     *slog.Logger
	ln      net.Listener
	srv     *http.Server
}

func ListenPhone(b PhoneBackend, addr, token string, log *slog.Logger) (*Phone, error) {
	if len(token) < 16 {
		return nil, errors.New("слишком короткий ключ ссылки для телефона")
	}
	if log == nil {
		log = slog.Default()
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	p := &Phone{backend: b, token: token, log: log, ln: ln}
	p.srv = &http.Server{
		Handler:           p.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		BaseContext:       func(net.Listener) context.Context { return context.Background() },
	}
	return p, nil
}

func (p *Phone) Port() int {
	if a, ok := p.ln.Addr().(*net.TCPAddr); ok {
		return a.Port
	}
	return 0
}

func (p *Phone) Path() string { return phonePrefix + p.token + "/" }

func (p *Phone) Serve() error {
	err := p.srv.Serve(p.ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (p *Phone) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return p.srv.Shutdown(ctx)
}

func (p *Phone) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /p/{key}", func(w http.ResponseWriter, r *http.Request) {
		if p.keyOK(r) {
			http.Redirect(w, r, p.Path(), http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("GET /p/{key}/{$}", p.only(p.page))
	mux.HandleFunc("GET /p/{key}/static/{file...}", p.only(p.static))
	mux.HandleFunc("GET /p/{key}/api/now", p.only(p.now))
	mux.HandleFunc("GET /p/{key}/api/history", p.only(p.history))
	mux.HandleFunc("GET /p/{key}/api/top", p.only(p.top))
	return p.guard(mux)
}

func (p *Phone) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !LocalNetwork(r.RemoteAddr) {
			http.Error(w, "доступ только из домашней сети", http.StatusForbidden)
			return
		}
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; "+
			"script-src 'self'; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (p *Phone) keyOK(r *http.Request) bool {
	return subtle.ConstantTimeCompare([]byte(r.PathValue("key")), []byte(p.token)) == 1
}

func (p *Phone) only(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !p.keyOK(r) {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}
}

func (p *Phone) page(w http.ResponseWriter, r *http.Request) {
	data, err := staticFiles.ReadFile("static/phone.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(themed(data, p.backend.State().Theme))
}

var phoneFiles = map[string]string{
	"phone.css": "text/css; charset=utf-8",
	"theme.css": "text/css; charset=utf-8",
	"phone.js":  "text/javascript; charset=utf-8",
	"glitch.js": "text/javascript; charset=utf-8",
	"icon.png":  "image/png",
}

func (p *Phone) static(w http.ResponseWriter, r *http.Request) {
	name := path.Clean(r.PathValue("file"))
	ct, ok := phoneFiles[name]
	if !ok && strings.HasPrefix(name, "fonts/") && strings.HasSuffix(name, ".woff2") {
		ct, ok = "font/woff2", true
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	data, err := staticFiles.ReadFile("static/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Write(data)
}

func (p *Phone) now(w http.ResponseWriter, r *http.Request) {
	st := p.backend.State()
	out := PhoneNow{User: st.User, Paused: st.Paused, Today: st.Today, Theme: Theme(st.Theme), Bursts: st.Bursts}
	if st.Now != nil {
		n := *st.Now
		n.Cover = ""
		out.Now = &n
	}
	writeJSON(w, out)
}

func (p *Phone) history(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 || limit > PhoneHistoryMax {
		limit = 50
	}
	list := p.backend.History(limit)
	if list == nil {
		list = []Scrobble{}
	}
	writeJSON(w, list)
}

func (p *Phone) top(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	switch period {
	case "week", "month", "year", "all":
	default:
		period = "week"
	}
	t, err := p.backend.Top("tracks", period, 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, t)
}

func LocalNetwork(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || cgnat.Contains(ip)
}

func QRRows(text string) ([]string, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return nil, err
	}
	const quiet = 2
	n := code.Size + 2*quiet
	rows := make([]string, n)
	var b strings.Builder
	for y := 0; y < n; y++ {
		b.Reset()
		for x := 0; x < n; x++ {
			if code.Black(x-quiet, y-quiet) {
				b.WriteByte('#')
			} else {
				b.WriteByte('.')
			}
		}
		rows[y] = b.String()
	}
	return rows, nil
}
