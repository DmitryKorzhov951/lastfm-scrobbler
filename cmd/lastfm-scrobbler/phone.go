package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/ui"
)

const defaultPhonePort = 47200

type phoneState struct {
	mu      sync.Mutex
	srv     *ui.Phone
	backend ui.PhoneBackend
	err     string
}

func newPhoneToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func (s *service) startPhone(b ui.PhoneBackend) {
	s.phone.mu.Lock()
	defer s.phone.mu.Unlock()
	s.phone.backend = b
	if s.config().PhoneAccess {
		s.openPhoneLocked()
	}
}

func (s *service) stopPhone() {
	s.phone.mu.Lock()
	defer s.phone.mu.Unlock()
	s.closePhoneLocked()
}

func (s *service) closePhoneLocked() {
	if s.phone.srv != nil {
		s.phone.srv.Close()
		s.phone.srv = nil
	}
}

func (s *service) openPhoneLocked() {
	s.phone.err = ""
	if s.phone.backend == nil {
		return
	}
	cfg := s.config()
	if cfg.PhoneToken == "" {
		cfg.PhoneToken = newPhoneToken()
	}
	port := cfg.PhonePort
	if port <= 0 {
		port = defaultPhonePort
	}
	srv, err := ui.ListenPhone(s.phone.backend, net.JoinHostPort("", strconv.Itoa(port)), cfg.PhoneToken, s.log)
	if err != nil {
		s.log.Info("порт для телефона занят — беру свободный", "port", port, "err", err)
		srv, err = ui.ListenPhone(s.phone.backend, ":0", cfg.PhoneToken, s.log)
	}
	if err != nil {
		s.phone.err = "не удалось открыть доступ для телефона: " + err.Error()
		s.log.Warn("страница для телефона не открыта", "err", err)
		return
	}
	cfg.PhonePort = srv.Port()
	if err := s.saveConfig(cfg); err != nil {
		s.log.Warn("не удалось сохранить настройки телефона", "err", err)
	}
	s.phone.srv = srv
	go func() {
		if err := srv.Serve(); err != nil {
			s.log.Warn("страница для телефона остановилась", "err", err)
		}
	}()
	s.log.Info("страница для телефона открыта", "port", srv.Port())
}

func (s *service) saveConfig(cfg config.Config) error {
	if err := config.Save(s.cfgPath, cfg); err != nil {
		return err
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return nil
}

func (s *service) phoneInfo() ui.PhoneInfo {
	s.phone.mu.Lock()
	defer s.phone.mu.Unlock()
	return s.phoneInfoLocked()
}

func (s *service) phoneInfoLocked() ui.PhoneInfo {
	info := ui.PhoneInfo{Enabled: s.config().PhoneAccess, Error: s.phone.err}
	if s.phone.srv == nil || !info.Enabled {
		return info
	}
	ips := lanIPs()
	if len(ips) == 0 {
		info.Error = "компьютер не подключён к сети — телефону не к чему подключиться"
		return info
	}
	for i, ip := range ips {
		u := fmt.Sprintf("http://%s%s", net.JoinHostPort(ip, strconv.Itoa(s.phone.srv.Port())), s.phone.srv.Path())
		if i == 0 {
			info.URL = u
		} else {
			info.Other = append(info.Other, u)
		}
	}
	return info
}

func (s *service) setPhone(enabled bool) (ui.PhoneInfo, error) {
	s.phone.mu.Lock()
	defer s.phone.mu.Unlock()
	cfg := s.config()
	if cfg.PhoneAccess != enabled {
		cfg.PhoneAccess = enabled
		if err := s.saveConfig(cfg); err != nil {
			return s.phoneInfoLocked(), fmt.Errorf("сохранение настроек: %w", err)
		}
	}
	switch {
	case enabled && s.phone.srv == nil:
		s.openPhoneLocked()
	case !enabled:
		s.closePhoneLocked()
		s.phone.err = ""
	}
	return s.phoneInfoLocked(), nil
}

func (s *service) resetPhone() (ui.PhoneInfo, error) {
	s.phone.mu.Lock()
	defer s.phone.mu.Unlock()
	cfg := s.config()
	cfg.PhoneToken = newPhoneToken()
	if err := s.saveConfig(cfg); err != nil {
		return s.phoneInfoLocked(), fmt.Errorf("сохранение настроек: %w", err)
	}
	if s.phone.srv != nil {
		s.closePhoneLocked()
		s.openPhoneLocked()
	}
	s.log.Info("ссылка для телефона заменена — старая больше не открывается")
	return s.phoneInfoLocked(), nil
}

func lanIPs() []string {
	var primary net.IP
	if c, err := net.Dial("udp4", "192.0.2.1:9"); err == nil {
		if a, ok := c.LocalAddr().(*net.UDPAddr); ok {
			primary = a.IP
		}
		c.Close()
	}
	var addrs []net.IP
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		list, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range list {
			if n, ok := a.(*net.IPNet); ok {
				addrs = append(addrs, n.IP)
			}
		}
	}
	return orderIPs(primary, addrs)
}

func orderIPs(primary net.IP, addrs []net.IP) []string {
	usable := func(ip net.IP) bool {
		ip4 := ip.To4()
		return ip4 != nil && !ip4.IsLoopback() && ui.LocalNetwork(ip4.String()) && !ip4.IsLinkLocalUnicast()
	}
	seen := map[string]bool{}
	var out []string
	if primary != nil && usable(primary) {
		out = append(out, primary.To4().String())
		seen[out[0]] = true
	}
	var rest []string
	for _, ip := range addrs {
		if !usable(ip) {
			continue
		}
		s := ip.To4().String()
		if !seen[s] {
			seen[s] = true
			rest = append(rest, s)
		}
	}
	sort.SliceStable(rest, func(i, j int) bool { return rank(rest[i]) < rank(rest[j]) })
	return append(out, rest...)
}

func rank(ip string) int {
	switch {
	case len(ip) > 8 && ip[:8] == "192.168.":
		return 0
	case len(ip) > 3 && ip[:3] == "10.":
		return 1
	default:
		return 2
	}
}
