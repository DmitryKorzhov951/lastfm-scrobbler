package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestПричинаСбояОкна(t *testing.T) {
	dir := t.TempDir()
	write := func(text string) string {
		p := filepath.Join(dir, "window.log")
		os.WriteFile(p, []byte(text), 0o600)
		return p
	}
	exit := errors.New("exit status 3")
	cases := []struct{ log, want string }{
		{"time=... level=INFO msg=\"найден WebView2\"\r\nОшибка: окно не загрузилось за 30 секунд\r\n", "окно не загрузилось за 30 секунд"},
		{"2026/11/06 18:00:01 Creating environment failed with 80070005\n", "WebView2 не запустился: Creating environment failed with 80070005"},
		{"panic: runtime error: invalid memory address\n\ngoroutine 1 [running]:\nmain.main()\n", "сбой процесса окна: runtime error: invalid memory address"},
		{"что-то пошло не так\n", "что-то пошло не так"},
		{"", "процесс окна завершился с ошибкой: exit status 3"},
	}
	for _, c := range cases {
		if got := windowError(write(c.log), exit); got != c.want {
			t.Errorf("журнал %q: причина %q, ожидалась %q", c.log, got, c.want)
		}
	}
	if got := windowError(filepath.Join(dir, "нет"), exit); !strings.Contains(got, "exit status 3") {
		t.Errorf("без журнала: %q", got)
	}
	long := windowError(write("Ошибка: "+strings.Repeat("я", 1000)), exit)
	if n := len([]rune(long)); n != 401 {
		t.Errorf("длинная причина не обрезана: %d символов", n)
	}
}
