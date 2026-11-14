package media

import (
	"net/url"
	"path/filepath"
	"strings"
)

func LocalPath(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(s), "file:") {
		u, err := url.Parse(s)
		if err != nil || u.Path == "" {
			return ""
		}
		p := u.Path
		if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
			p = p[1:]
		}
		if u.Host != "" && u.Host != "localhost" {
			p = "//" + u.Host + p
		}
		return filepath.FromSlash(p)
	}
	if filepath.IsAbs(s) {
		return s
	}
	return ""
}
