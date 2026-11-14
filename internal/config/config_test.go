package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestDurationJSON(t *testing.T) {
	var v struct {
		D Duration `json:"d"`
	}
	if err := json.Unmarshal([]byte(`{"d":"4m30s"}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.D.Duration() != 4*time.Minute+30*time.Second {
		t.Errorf("разбор строки: %s", v.D.Duration())
	}
	if err := json.Unmarshal([]byte(`{"d":90}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.D.Duration() != 90*time.Second {
		t.Errorf("разбор числа: %s", v.D.Duration())
	}

	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"d":"1m30s"}` {
		t.Errorf("сериализация = %s", out)
	}

	if err := json.Unmarshal([]byte(`{"d":"вчера"}`), &v); err == nil {
		t.Error("ожидалась ошибка разбора")
	}
}

func TestLoadОтсутствующийФайл(t *testing.T) {
	t.Setenv("LASTFM_API_KEY", "")
	t.Setenv("LASTFM_API_SECRET", "")

	cfg, err := Load(filepath.Join(t.TempDir(), "нет.json"))
	if err != nil {
		t.Fatalf("отсутствие файла не должно быть ошибкой: %v", err)
	}
	if cfg.ScrobblePercent != 0.5 || cfg.MaxPlayTime.Duration() != 4*time.Minute {
		t.Errorf("не применены умолчания: %+v", cfg)
	}
}

func TestLoadИSave(t *testing.T) {
	t.Setenv("LASTFM_API_KEY", "")
	t.Setenv("LASTFM_API_SECRET", "")

	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Default()
	cfg.APIKey = "KEY"
	cfg.APISecret = "SECRET"
	cfg.IgnoredPlayers = []string{"firefox"}
	cfg.MaxPlayTime = Duration(2 * time.Minute)

	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm != 0o600 {
		t.Errorf("права файла настроек = %o, ожидалось 600 (в нём лежит api_secret)", perm)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "KEY" || got.APISecret != "SECRET" {
		t.Errorf("ключи не прочитаны: %+v", got)
	}
	if got.MaxPlayTime.Duration() != 2*time.Minute {
		t.Errorf("MaxPlayTime = %s", got.MaxPlayTime.Duration())
	}
	if len(got.IgnoredPlayers) != 1 || got.IgnoredPlayers[0] != "firefox" {
		t.Errorf("IgnoredPlayers = %v", got.IgnoredPlayers)
	}
}

func TestПеременныеОкруженияПерекрываютФайл(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Default()
	cfg.APIKey = "из-файла"
	cfg.APISecret = "из-файла"
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LASTFM_API_KEY", "из-окружения")
	t.Setenv("LASTFM_API_SECRET", "секрет-из-окружения")

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "из-окружения" || got.APISecret != "секрет-из-окружения" {
		t.Errorf("переменные окружения не применены: %+v", got)
	}
}

func TestValidateПравитБессмыслицу(t *testing.T) {
	cfg := Config{
		PollInterval:    Duration(time.Millisecond),
		ScrobblePercent: 42,
		MinTrackLength:  Duration(-time.Second),
		RetryInterval:   Duration(time.Second),
		QueueLimit:      -5,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.PollInterval.Duration() != 2*time.Second {
		t.Errorf("PollInterval = %s", cfg.PollInterval.Duration())
	}
	if cfg.ScrobblePercent != 0.5 {
		t.Errorf("ScrobblePercent = %v", cfg.ScrobblePercent)
	}
	if cfg.MinTrackLength.Duration() != 30*time.Second {
		t.Errorf("MinTrackLength = %s", cfg.MinTrackLength.Duration())
	}
	if cfg.RetryInterval.Duration() != time.Minute {
		t.Errorf("RetryInterval = %s", cfg.RetryInterval.Duration())
	}
	if cfg.QueueLimit != 5000 {
		t.Errorf("QueueLimit = %d", cfg.QueueLimit)
	}
}

func TestСессия(t *testing.T) {
	t.Setenv("LASTFM_SESSION_KEY", "")

	path := filepath.Join(t.TempDir(), "session.json")
	s := Session{Username: "пользователь", Key: "SK", CreatedAt: time.Now().UTC().Truncate(time.Second)}
	if err := SaveSession(path, s); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm != 0o600 {
		t.Errorf("права файла сессии = %o, ожидалось 600", perm)
	}

	got, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "SK" || got.Username != "пользователь" {
		t.Errorf("сессия прочитана неверно: %+v", got)
	}

	t.Setenv("LASTFM_SESSION_KEY", "временный-ключ")
	got, err = LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "временный-ключ" {
		t.Errorf("LASTFM_SESSION_KEY должен перекрывать файл, получено %q", got.Key)
	}
}

func TestПутиWindows(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(dir, "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "Local"))

	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "Roaming", AppName, "config.json"); p != want {
		t.Errorf("Path() = %s, ожидалось %s", p, want)
	}
	q, err := QueuePath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "Local", AppName, "queue.json"); q != want {
		t.Errorf("QueuePath() = %s, ожидалось %s", q, want)
	}
}

func TestБраузерыПоУмолчанию(t *testing.T) {
	if ignored := Default().IgnoredPlayers; len(ignored) != 0 {
		t.Errorf("браузеры — частый источник музыки, по умолчанию их отключать нельзя: %v", ignored)
	}
}

func TestРывкиВключеныДажеВСтаромФайле(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.json")
	if err := os.WriteFile(old, []byte(`{"theme":"glitch"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(old)
	if err != nil || !cfg.GlitchBursts {
		t.Fatalf("старый файл без поля: рывки %v, %v", cfg.GlitchBursts, err)
	}
	cfg.GlitchBursts = false
	path := filepath.Join(dir, "config.json")
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	if again, err := Load(path); err != nil || again.GlitchBursts {
		t.Errorf("выключенные рывки не сохранились: %v, %v", again.GlitchBursts, err)
	}
}
