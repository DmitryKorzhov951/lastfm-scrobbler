package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestТемаТолькоИзСписка(t *testing.T) {
	for in, want := range map[string]string{
		"": "standard", "standard": "standard", "dark": "dark", "brat": "brat", "glitch": "glitch", "fallen": "fallen",
		"Dark": "standard", "Fallen": "standard", `"><script>`: "standard",
	} {
		if got := Theme(in); got != want {
			t.Errorf("Theme(%q) = %q, ожидалось %q", in, got, want)
		}
	}
	page := []byte(`<!doctype html><html lang="ru"><head>`)
	if got := string(themed(page, `"><script>`)); got != `<!doctype html><html lang="ru" data-theme="standard"><head>` {
		t.Errorf("в страницу попало чужое: %s", got)
	}
}

func TestТемаСохраняетсяИСразуВСтранице(t *testing.T) {
	d := NewDemo(true, false)
	s, h := testServer(t, d)
	if rec := do(t, h, s, "GET", "/", "", false); !strings.Contains(rec.Body.String(), `<html lang="ru" data-theme="standard">`) {
		t.Fatalf("по умолчанию стандартная тема: %s", rec.Body.String()[:120])
	}
	if rec := do(t, h, s, "POST", "/api/settings", `{"percent":0.5,"theme":"dark"}`, true); rec.Code != http.StatusOK {
		t.Fatalf("сохранение темы: код %d, %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, s, "GET", "/", "", false); !strings.Contains(rec.Body.String(), `data-theme="dark"`) {
		t.Error("страница окна сразу приходит в тёмной теме")
	}
	var st State
	if rec := do(t, h, s, "GET", "/api/state", "", true); json.Unmarshal(rec.Body.Bytes(), &st) != nil || st.Theme != "dark" {
		t.Errorf("тема в состоянии: %q", st.Theme)
	}
	if rec := do(t, h, s, "POST", "/api/settings", `{"percent":0.5,"theme":"neon"}`, true); rec.Code != http.StatusBadRequest {
		t.Errorf("неизвестная тема: код %d", rec.Code)
	}
	if rec := do(t, h, s, "POST", "/api/settings", `{"percent":0.5}`, true); rec.Code != http.StatusOK {
		t.Errorf("настройки без темы: код %d", rec.Code)
	}
	if got := d.Settings().Theme; got != "dark" {
		t.Errorf("настройки без темы не сбрасывают её: %q", got)
	}

	p, err := ListenPhone(d, "127.0.0.1:0", testPhoneToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.ln.Close() })
	if rec := phoneGet(p, "GET", p.Path(), "192.168.1.20:5000"); !strings.Contains(rec.Body.String(), `data-theme="dark"`) {
		t.Error("страница телефона в той же теме")
	}
	var now PhoneNow
	if rec := phoneGet(p, "GET", p.Path()+"api/now", "192.168.1.20:5000"); json.Unmarshal(rec.Body.Bytes(), &now) != nil || now.Theme != "dark" {
		t.Errorf("тема для телефона: %q", now.Theme)
	}
	if rec := phoneGet(p, "GET", p.Path()+"static/theme.css", "192.168.1.20:5000"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `[data-theme="brat"]`) ||
		!strings.Contains(rec.Body.String(), `[data-theme="fallen"]`) {
		t.Errorf("телефон получает файл тем: код %d", rec.Code)
	}
	if rec := do(t, h, s, "POST", "/api/settings", `{"percent":0.5,"theme":"fallen"}`, true); rec.Code != http.StatusOK {
		t.Fatalf("тема fallen: код %d, %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, s, "GET", "/", "", false); !strings.Contains(rec.Body.String(), `data-theme="fallen"`) || !strings.Contains(rec.Body.String(), "skin-fallen.css") {
		t.Error("страница окна приходит в теме fallen и со своими стилями")
	}
}

func TestРывкиВыключаютсяИВОкнеИНаТелефоне(t *testing.T) {
	d := NewDemo(true, false)
	s, h := testServer(t, d)
	var st State
	if rec := do(t, h, s, "GET", "/api/state", "", true); json.Unmarshal(rec.Body.Bytes(), &st) != nil || !st.Bursts {
		t.Fatalf("по умолчанию рывки включены: %+v", st.Bursts)
	}
	if rec := do(t, h, s, "POST", "/api/settings", `{"percent":0.5,"bursts":false}`, true); rec.Code != http.StatusOK {
		t.Fatalf("сохранение: код %d", rec.Code)
	}
	if rec := do(t, h, s, "POST", "/api/settings", `{"percent":0.5}`, true); rec.Code != http.StatusOK {
		t.Fatalf("сохранение без флажка: код %d", rec.Code)
	}
	if rec := do(t, h, s, "GET", "/api/state", "", true); json.Unmarshal(rec.Body.Bytes(), &st) != nil || st.Bursts {
		t.Errorf("рывки остались выключены: %v", st.Bursts)
	}
	p, err := ListenPhone(d, "127.0.0.1:0", testPhoneToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.ln.Close() })
	var now PhoneNow
	if rec := phoneGet(p, "GET", p.Path()+"api/now", "192.168.1.20:5000"); json.Unmarshal(rec.Body.Bytes(), &now) != nil || now.Bursts {
		t.Errorf("телефон знает, что рывки выключены: %v", now.Bursts)
	}
}
