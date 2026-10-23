package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	AppName     = "lastfm-scrobbler"
	configFile  = "config.json"
	sessionFile = "session.json"
	queueFile   = "queue.json"
	historyFile = "history.json"
	windowFile  = "window.json"
	radioFile   = "radio.json"
)

type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*d = 0
		return nil
	}
	if v, err := time.ParseDuration(s); err == nil {
		*d = Duration(v)
		return nil
	}
	secs, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("не удалось разобрать длительность %q", s)
	}
	*d = Duration(time.Duration(secs * float64(time.Second)))
	return nil
}

func (d Duration) Duration() time.Duration { return time.Duration(d) }

type Config struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`

	PollInterval Duration `json:"poll_interval"`

	ScrobblePercent   float64  `json:"scrobble_percent"`
	MinTrackLength    Duration `json:"min_track_length"`
	MaxPlayTime       Duration `json:"max_play_time"`
	SkipUnknownLength bool     `json:"skip_unknown_length"`

	AllowedPlayers []string `json:"allowed_players"`
	IgnoredPlayers []string `json:"ignored_players"`

	RetryInterval Duration `json:"retry_interval"`
	QueueLimit    int      `json:"queue_limit"`

	LogLevel string `json:"log_level"`
}

func Default() Config {
	return Config{
		PollInterval:      Duration(2 * time.Second),
		ScrobblePercent:   0.5,
		MinTrackLength:    Duration(30 * time.Second),
		MaxPlayTime:       Duration(4 * time.Minute),
		SkipUnknownLength: false,
		IgnoredPlayers:    defaultIgnored(),
		RetryInterval:     Duration(time.Minute),
		QueueLimit:        5000,
		LogLevel:          "info",
	}
}

func defaultIgnored() []string {
	if runtime.GOOS == "windows" {
		return []string{}
	}
	return []string{"firefox", "chromium", "google-chrome", "kdeconnect"}
}

func (c *Config) applyEnv() {
	if v := os.Getenv("LASTFM_API_KEY"); v != "" {
		c.APIKey = v
	}
	if v := os.Getenv("LASTFM_API_SECRET"); v != "" {
		c.APISecret = v
	}
	if v := os.Getenv("LASTFM_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
}

func (c *Config) Validate() error {
	d := Default()
	if c.PollInterval.Duration() < time.Second {
		c.PollInterval = d.PollInterval
	}
	if c.ScrobblePercent <= 0 || c.ScrobblePercent > 1 {
		c.ScrobblePercent = d.ScrobblePercent
	}
	if c.MinTrackLength.Duration() <= 0 {
		c.MinTrackLength = d.MinTrackLength
	}
	if c.MaxPlayTime.Duration() <= 0 {
		c.MaxPlayTime = d.MaxPlayTime
	}
	if c.RetryInterval.Duration() < 10*time.Second {
		c.RetryInterval = d.RetryInterval
	}
	if c.QueueLimit <= 0 {
		c.QueueLimit = d.QueueLimit
	}
	if c.LogLevel == "" {
		c.LogLevel = d.LogLevel
	}
	return nil
}

func Dir() (string, error) {
	if runtime.GOOS == "windows" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("не удалось определить каталог %%APPDATA%%: %w", err)
		}
		return filepath.Join(base, AppName), nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("не удалось определить домашний каталог: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, AppName), nil
}

func StateDir() (string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			var err error
			if base, err = os.UserCacheDir(); err != nil {
				return "", fmt.Errorf("не удалось определить каталог %%LOCALAPPDATA%%: %w", err)
			}
		}
		return filepath.Join(base, AppName), nil
	}
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("не удалось определить домашний каталог: %w", err)
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, AppName), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFile), nil
}

func SessionPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, sessionFile), nil
}

func QueuePath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, queueFile), nil
}

func HistoryPath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, historyFile), nil
}

func WindowPath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, windowFile), nil
}

func RadioPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, radioFile), nil
}

func DeleteSession(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("разбор %s: %w", path, err)
		}
	case os.IsNotExist(err):
	default:
		return cfg, fmt.Errorf("чтение %s: %w", path, err)
	}

	cfg.applyEnv()
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return writeFilePrivate(path, append(data, '\n'))
}

type Session struct {
	Username  string    `json:"username"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
}

func LoadSession(path string) (Session, error) {
	var s Session
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &s); err != nil {
			return s, fmt.Errorf("разбор %s: %w", path, err)
		}
	case os.IsNotExist(err):
	default:
		return s, fmt.Errorf("чтение %s: %w", path, err)
	}
	if v := os.Getenv("LASTFM_SESSION_KEY"); v != "" {
		s.Key = v
	}
	return s, nil
}

func SaveSession(path string, s Session) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeFilePrivate(path, append(data, '\n'))
}

func writeFilePrivate(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("создание каталога %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
