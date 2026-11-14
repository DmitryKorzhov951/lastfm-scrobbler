package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/library"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/radio"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/ui"
)

const keysPage = "https://www.last.fm/api/account/create"

type uiBackend struct {
	svc *service
	env *env
}

var _ ui.Backend = (*uiBackend)(nil)

func statusWord(s media.Status) string {
	switch s {
	case media.Playing:
		return "playing"
	case media.Paused:
		return "paused"
	default:
		return "stopped"
	}
}

func (b *uiBackend) State() ui.State {
	svc := b.svc
	acc := svc.account()
	cfg := svc.config()
	link := svc.sub.Status()

	st := ui.State{
		HasKeys:  cfg.APIKey != "" && cfg.APISecret != "",
		LoggedIn: link.LoggedIn,
		User:     acc.Username,
		Since:    acc.CreatedAt,
		Paused:   svc.Paused(),
		QueueLen: svc.queue.Len(),
		Theme:    ui.Theme(cfg.Theme),
		Bursts:   cfg.GlitchBursts,
		Closed:   append([]string{}, cfg.ClosedPanels...),
	}
	switch {
	case !link.LoggedIn:
		st.Link = ui.Link{State: "nologin"}
	case link.AuthErr:
		st.Link = ui.Link{State: "auth", Error: link.LastErr}
	case link.Failing:
		st.Link = ui.Link{State: "offline", Error: link.LastErr, RetryAt: link.NextTry}
	default:
		st.Link = ui.Link{State: "ok"}
	}

	svc.mu.Lock()
	if svc.login != nil {
		st.Login = &ui.LoginState{URL: svc.login.url}
	}
	svc.mu.Unlock()

	if sn, ok := svc.activeSnapshot(); ok && sn.Track.Valid() {
		player := sn.Identity
		if player == "" {
			player = sn.Player
		}
		st.Now = &ui.NowPlaying{
			Player:      player,
			Status:      statusWord(sn.Status),
			Artist:      sn.Track.Artist(),
			Title:       sn.Track.Title,
			Album:       sn.Track.Album,
			Length:      sn.Track.Length.Seconds(),
			Played:      sn.Played.Seconds(),
			Position:    sn.Position.Seconds(),
			Synced:      sn.Synced,
			Target:      sn.Target.Seconds(),
			Left:        sn.Left.Seconds(),
			Scrobbled:   sn.Scrobbld,
			Scrobblable: sn.Target > 0,
			Live:        sn.Track.Live,
			FromStart:   sn.FromStart,
			Charts:      svc.ranks(sn.Track.Artist(), sn.Track.Title, sn.Track.Album),
			Loved:       svc.loved(sn.Track.Artist(), sn.Track.Title, sn.Status == media.Playing),
			Moment:      svc.moment(sn.Track),
			Tags:        svc.tags(sn.Track.Artist(), sn.Track.Title),
		}
	}

	r := svc.radio.Status()
	st.Radio = ui.RadioState{
		Playing: r.Playing, Station: r.Station.ID, Name: r.Station.Name,
		Connected: r.Connected, Title: r.Title, Artist: r.Artist, Song: r.Song,
		Format: r.Format, Error: r.Error, Volume: r.Volume, Last: r.Last,
		Count: len(svc.radio.Stations()),
	}

	tot := svc.hist.Totals(time.Now())
	st.Today = ui.Totals{Count: tot.TodayCount, Seconds: tot.TodayTime.Seconds()}
	st.All = ui.Totals{Count: tot.AllCount, Seconds: tot.AllTime.Seconds()}
	st.Recent = svc.recentScrobbles(10)
	st.Library = svc.libSync.Status()
	switch {
	case svc.localMode():
		st.All.Count = svc.archive.Len()
	case st.Library.Plays > 0:
		st.All.Count = svc.lib.Total(svc.localPlays())
	}
	return st
}

func (b *uiBackend) Top(kind, period string, offset int) (library.Top, error) {
	lib, extra := b.svc.stats()
	return lib.Top(kind, period, offset, time.Now(), extra)
}

func (b *uiBackend) Card(kind, artist, title, album string) (library.Card, error) {
	lib, extra := b.svc.stats()
	return lib.Card(kind, artist, title, album, time.Now(), extra)
}

func (b *uiBackend) ReloadLibrary() error {
	if b.svc.account().Username == "" {
		return errors.New("вход не выполнен")
	}
	return b.svc.libSync.Reload()
}

func (b *uiBackend) Queue() []ui.QueueItem {
	items := b.svc.queue.Items()
	out := make([]ui.QueueItem, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		it := items[i]
		out = append(out, ui.QueueItem{
			ID: it.ID, Started: it.Scrobble.Timestamp, Artist: it.Scrobble.Artist,
			Title: it.Scrobble.Title, Attempts: it.Attempts,
		})
	}
	return out
}

func (b *uiBackend) DeleteQueued(ids []int64) error { return b.svc.DeleteQueued(ids...) }
func (b *uiBackend) ClearQueue() error              { return b.svc.ClearQueue() }
func (b *uiBackend) FlushQueue()                    { b.svc.sub.RequestFlush() }
func (b *uiBackend) SetPaused(p bool)               { b.svc.SetPaused(p) }
func (b *uiBackend) SaveKeys(k, s string) error     { return b.svc.SaveKeys(k, s) }
func (b *uiBackend) Logout() error                  { return b.svc.Logout() }

func (b *uiBackend) Settings() ui.Settings {
	cfg := b.svc.config()
	playing := ""
	if sn, ok := b.svc.activeSnapshot(); ok && sn.Status == media.Playing {
		playing = sn.Player
	}
	s := ui.Settings{
		Percent:      cfg.ScrobblePercent,
		MaxPlay:      cfg.MaxPlayTime.Duration().Seconds(),
		MinLength:    cfg.MinTrackLength.Duration().Seconds(),
		Radio:        !cfg.SkipUnknownLength,
		Autostart:    autostartEnabled(),
		CanAutostart: autostartSupported,
		LogDir:       logDir(),
		Theme:        ui.Theme(cfg.Theme),
		Bursts:       &cfg.GlitchBursts,
		LyricsDir:    cfg.LyricsPath(),
		LyricsAuto:   config.DefaultLyricsFolder(),
	}
	for _, p := range b.svc.hist.Players() {
		s.Players = append(s.Players, ui.PlayerSetting{
			Name:     p.Name,
			Identity: p.Identity,
			Enabled:  playerAllowed(p.Name, cfg.AllowedPlayers, cfg.IgnoredPlayers),
			Playing:  strings.EqualFold(p.Name, playing),
		})
	}
	return s
}

func (b *uiBackend) SaveSettings(ctx context.Context, s ui.Settings) error {
	cfg := b.svc.config()
	cfg.ScrobblePercent = s.Percent
	cfg.SkipUnknownLength = !s.Radio
	if s.Theme != "" {
		cfg.Theme = ui.Theme(s.Theme)
	}
	if s.Bursts != nil {
		cfg.GlitchBursts = *s.Bursts
	}
	folder, err := lyricsFolderSetting(s.LyricsDir)
	if err != nil {
		return err
	}
	cfg.LyricsFolder = folder
	for _, p := range s.Players {
		if p.Enabled {
			cfg.IgnoredPlayers = without(cfg.IgnoredPlayers, p.Name)
			if len(cfg.AllowedPlayers) > 0 {
				cfg.AllowedPlayers = with(cfg.AllowedPlayers, p.Name)
			}
		} else {
			cfg.IgnoredPlayers = with(cfg.IgnoredPlayers, p.Name)
			cfg.AllowedPlayers = without(cfg.AllowedPlayers, p.Name)
		}
	}
	if err := b.svc.ApplyConfig(ctx, cfg); err != nil {
		return err
	}
	if autostartSupported && s.Autostart != autostartEnabled() {
		return setAutostart(b.env, s.Autostart)
	}
	return nil
}

func (b *uiBackend) StartLogin(ctx context.Context) (string, error) {
	u, err := b.svc.StartLogin(ctx)
	if err != nil {
		return "", err
	}
	if err := openURL(u); err != nil {
		b.svc.log.Warn("не удалось открыть браузер", "err", err)
	}
	return u, nil
}

func (b *uiBackend) CheckLogin(ctx context.Context) (bool, error) {
	_, err := b.svc.CheckLogin(ctx)
	if errors.Is(err, errLoginPending) {
		return false, nil
	}
	return err == nil, err
}

func (b *uiBackend) Open(what string) error {
	switch what {
	case "profile":
		user := b.svc.account().Username
		if user == "" {
			return errors.New("вход не выполнен")
		}
		return openURL("https://www.last.fm/user/" + url.PathEscape(user))
	case "keys":
		return openURL(keysPage)
	case "lyrics":
		return b.openLyricsSearch()
	case "lyricsdir":
		return b.openLyricsFolder()
	case "logs":
		return openFolder(logDir())
	case "auth":
		b.svc.mu.Lock()
		l := b.svc.login
		b.svc.mu.Unlock()
		if l == nil {
			return errors.New("вход не начат")
		}
		return openURL(l.url)
	}
	return errors.New("неизвестная страница: " + what)
}

func with(list []string, name string) []string {
	for _, s := range list {
		if strings.EqualFold(s, name) {
			return list
		}
	}
	return append(list, name)
}

func without(list []string, name string) []string {
	out := list[:0:0]
	for _, s := range list {
		if !strings.EqualFold(s, name) {
			out = append(out, s)
		}
	}
	return out
}

func uiStation(s radio.Station) ui.Station { return ui.Station{ID: s.ID, Name: s.Name, URL: s.URL} }

func (b *uiBackend) Stations() []ui.Station {
	list := b.svc.radio.Stations()
	out := make([]ui.Station, 0, len(list))
	for _, s := range list {
		out = append(out, uiStation(s))
	}
	return out
}

func (b *uiBackend) AddStation(name, rawURL string) (ui.Station, error) {
	s, err := b.svc.radio.Add(name, rawURL)
	return uiStation(s), err
}

func (b *uiBackend) EditStation(id, name, rawURL string) error {
	return b.svc.radio.Edit(id, name, rawURL)
}
func (b *uiBackend) DeleteStation(id string) error   { return b.svc.radio.Delete(id) }
func (b *uiBackend) PlayRadio(id string) error       { return b.svc.radio.Play(id) }
func (b *uiBackend) StopRadio()                      { b.svc.radio.Stop() }
func (b *uiBackend) SetRadioVolume(volume int) error { return b.svc.radio.SetVolume(volume) }
func (b *uiBackend) RadioStream(w http.ResponseWriter, r *http.Request, id string) {
	b.svc.radio.ServeStream(w, r, id)
}

func (b *uiBackend) NowCover() string {
	sn, ok := b.svc.activeSnapshot()
	if !ok || !sn.Track.Valid() {
		return ""
	}
	return b.svc.covers.Key(sn.ID, sn.Track)
}

func (b *uiBackend) Cover(key string) ([]byte, string, bool) {
	img, ok := b.svc.covers.Image(key)
	return img.Data, img.Type, ok
}

func (b *uiBackend) History(limit int) []ui.Scrobble { return b.svc.recentScrobbles(limit) }

func (b *uiBackend) SyncLibrary() { b.svc.libSync.SyncNow() }

func (b *uiBackend) Phone() ui.PhoneInfo { return b.svc.phoneInfo() }

func (b *uiBackend) SetPhone(enabled bool) (ui.PhoneInfo, error) { return b.svc.setPhone(enabled) }

func (b *uiBackend) ResetPhone() (ui.PhoneInfo, error) { return b.svc.resetPhone() }
