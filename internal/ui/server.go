package ui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed static
var staticFiles embed.FS

type Server struct {
	backend Backend
	token   string
	ln      net.Listener
	srv     *http.Server
	port    string
	log     *slog.Logger
}

func Listen(b Backend, addr string, log *slog.Logger) (*Server, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return nil, errors.New("окно программы доступно только с этого компьютера: адрес должен быть 127.0.0.1")
	}
	if log == nil {
		log = slog.Default()
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	_, port, _ := net.SplitHostPort(ln.Addr().String())

	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		ln.Close()
		return nil, err
	}
	s := &Server{backend: b, token: hex.EncodeToString(buf), ln: ln, port: port, log: log}
	s.srv = &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return context.Background() },
	}
	return s, nil
}

func (s *Server) URL() string {
	return "http://127.0.0.1:" + s.port + "/#t=" + s.token
}

func (s *Server) Serve() error {
	err := s.srv.Serve(s.ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}

func (s *Server) Handler() http.Handler {
	static, _ := fs.Sub(staticFiles, "static")
	files := http.FileServer(http.FS(static))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
	mux.Handle("GET /static/", http.StripPrefix("/static/", files))

	api := http.NewServeMux()
	api.HandleFunc("GET /api/state", s.getState)
	api.HandleFunc("GET /api/queue", s.getQueue)
	api.HandleFunc("POST /api/queue/delete", s.deleteQueued)
	api.HandleFunc("POST /api/queue/clear", s.clearQueue)
	api.HandleFunc("POST /api/queue/flush", s.flushQueue)
	api.HandleFunc("POST /api/pause", s.setPaused)
	api.HandleFunc("GET /api/settings", s.getSettings)
	api.HandleFunc("POST /api/settings", s.saveSettings)
	api.HandleFunc("POST /api/keys", s.saveKeys)
	api.HandleFunc("POST /api/login/start", s.startLogin)
	api.HandleFunc("POST /api/login/check", s.checkLogin)
	api.HandleFunc("POST /api/logout", s.logout)
	api.HandleFunc("POST /api/open", s.open)
	api.HandleFunc("GET /api/radio/stations", s.getStations)
	api.HandleFunc("POST /api/radio/stations", s.addStation)
	api.HandleFunc("POST /api/radio/stations/edit", s.editStation)
	api.HandleFunc("POST /api/radio/stations/delete", s.deleteStation)
	api.HandleFunc("POST /api/radio/play", s.playRadio)
	api.HandleFunc("POST /api/radio/stop", s.stopRadio)
	api.HandleFunc("POST /api/radio/volume", s.radioVolume)
	mux.Handle("/api/", s.requireToken(api))

	mux.HandleFunc("GET /radio/stream", s.radioStream)

	return s.guard(mux)
}

func (s *Server) guard(next http.Handler) http.Handler {
	allowed := map[string]bool{
		"127.0.0.1:" + s.port: true,
		"localhost:" + s.port: true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowed[r.Host] {
			http.Error(w, "неверный адрес", http.StatusMisdirectedRequest)
			return
		}
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; "+
			"style-src 'self'; script-src 'self'; font-src 'self'; connect-src 'self'; "+
			"frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireToken(next http.Handler) http.Handler {
	want := []byte(s.token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("X-Token"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			writeError(w, http.StatusUnauthorized, errors.New("окно открыто без ключа доступа — откройте его через значок программы"))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func readJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return errors.New("неверный запрос: " + err.Error())
	}
	return nil
}

func ok(w http.ResponseWriter) { writeJSON(w, map[string]bool{"ok": true}) }

func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.backend.State())
}

func (s *Server) getQueue(w http.ResponseWriter, r *http.Request) {
	items := s.backend.Queue()
	if items == nil {
		items = []QueueItem{}
	}
	writeJSON(w, items)
}

func (s *Server) deleteQueued(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.backend.DeleteQueued(req.IDs); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	ok(w)
}

func (s *Server) clearQueue(w http.ResponseWriter, r *http.Request) {
	if err := s.backend.ClearQueue(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	ok(w)
}

func (s *Server) flushQueue(w http.ResponseWriter, r *http.Request) {
	s.backend.FlushQueue()
	ok(w)
}

func (s *Server) setPaused(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paused bool `json:"paused"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.backend.SetPaused(req.Paused)
	ok(w)
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	st := s.backend.Settings()
	if st.Players == nil {
		st.Players = []PlayerSetting{}
	}
	writeJSON(w, st)
}

func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	var st Settings
	if err := readJSON(r, &st); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if st.Percent <= 0 || st.Percent > 1 {
		writeError(w, http.StatusBadRequest, errors.New("доля трека должна быть от 1 до 100 %: "+strconv.FormatFloat(st.Percent*100, 'f', 0, 64)))
		return
	}
	if err := s.backend.SaveSettings(r.Context(), st); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	ok(w)
}

func (s *Server) saveKeys(w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIKey string `json:"apiKey"`
		Secret string `json:"secret"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.backend.SaveKeys(strings.TrimSpace(req.APIKey), strings.TrimSpace(req.Secret)); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ok(w)
}

func (s *Server) startLogin(w http.ResponseWriter, r *http.Request) {
	url, err := s.backend.StartLogin(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, map[string]string{"url": url})
}

func (s *Server) checkLogin(w http.ResponseWriter, r *http.Request) {
	done, err := s.backend.CheckLogin(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, map[string]bool{"done": done})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.backend.Logout(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	ok(w)
}

func (s *Server) open(w http.ResponseWriter, r *http.Request) {
	var req struct {
		What string `json:"what"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	switch req.What {
	case "profile", "keys", "logs", "auth":
	default:
		writeError(w, http.StatusBadRequest, errors.New("неизвестная страница: "+req.What))
		return
	}
	if err := s.backend.Open(req.What); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	ok(w)
}

func (s *Server) getStations(w http.ResponseWriter, r *http.Request) {
	list := s.backend.Stations()
	if list == nil {
		list = []Station{}
	}
	writeJSON(w, list)
}

type stationRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (s *Server) addStation(w http.ResponseWriter, r *http.Request) {
	var req stationRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	st, err := s.backend.AddStation(req.Name, req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, st)
}

func (s *Server) editStation(w http.ResponseWriter, r *http.Request) {
	var req stationRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.backend.EditStation(req.ID, req.Name, req.URL); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ok(w)
}

func (s *Server) deleteStation(w http.ResponseWriter, r *http.Request) {
	var req stationRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.backend.DeleteStation(req.ID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ok(w)
}

func (s *Server) playRadio(w http.ResponseWriter, r *http.Request) {
	var req stationRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.backend.PlayRadio(req.ID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ok(w)
}

func (s *Server) stopRadio(w http.ResponseWriter, r *http.Request) {
	s.backend.StopRadio()
	ok(w)
}

func (s *Server) radioVolume(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Volume int `json:"volume"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Volume < 0 || req.Volume > 100 {
		writeError(w, http.StatusBadRequest, errors.New("громкость — от 0 до 100"))
		return
	}
	if err := s.backend.SetRadioVolume(req.Volume); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	ok(w)
}

func (s *Server) radioStream(w http.ResponseWriter, r *http.Request) {
	got := []byte(r.URL.Query().Get("t"))
	if subtle.ConstantTimeCompare(got, []byte(s.token)) != 1 {
		http.Error(w, "нет доступа", http.StatusUnauthorized)
		return
	}
	s.backend.RadioStream(w, r, r.URL.Query().Get("s"))
}
