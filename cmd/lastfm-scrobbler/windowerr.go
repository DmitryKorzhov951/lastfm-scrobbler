package main

import (
	"os"
	"regexp"
	"strings"
)

var timestampPrefix = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} `)

func windowError(logPath string, err error) string {
	var lines []string
	if data, rerr := os.ReadFile(logPath); rerr == nil {
		for _, ln := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			if ln = strings.TrimSpace(ln); ln != "" {
				lines = append(lines, ln)
			}
		}
	}
	pick := ""
	for i := len(lines) - 1; i >= 0 && pick == ""; i-- {
		switch ln := lines[i]; {
		case strings.HasPrefix(ln, "Ошибка: "):
			pick = strings.TrimPrefix(ln, "Ошибка: ")
		case strings.HasPrefix(ln, "panic: "):
			pick = "сбой процесса окна: " + strings.TrimPrefix(ln, "panic: ")
		case strings.Contains(ln, "failed with"):
			pick = "WebView2 не запустился: " + timestampPrefix.ReplaceAllString(ln, "")
		}
	}
	if pick == "" && len(lines) > 0 {
		pick = timestampPrefix.ReplaceAllString(lines[len(lines)-1], "")
	}
	if pick == "" {
		pick = "процесс окна завершился с ошибкой: " + err.Error()
	}
	if r := []rune(pick); len(r) > 400 {
		pick = string(r[:400]) + "…"
	}
	return pick
}
