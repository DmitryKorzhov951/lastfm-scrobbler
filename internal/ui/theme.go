package ui

import "bytes"

const DefaultTheme = "standard"

var themes = map[string]bool{"standard": true, "dark": true, "brat": true, "glitch": true, "fallen": true}

func Theme(name string) string {
	if themes[name] {
		return name
	}
	return DefaultTheme
}

func themed(page []byte, theme string) []byte {
	return bytes.Replace(page, []byte(`<html lang="ru">`), []byte(`<html lang="ru" data-theme="`+Theme(theme)+`">`), 1)
}
