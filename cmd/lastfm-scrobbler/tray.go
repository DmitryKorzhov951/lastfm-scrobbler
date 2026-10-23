package main

import (
	"context"
	"embed"
	"fmt"
	"runtime"
	"time"
	"unicode/utf8"

	"fyne.io/systray"
)

//go:embed assets
var assets embed.FS

func trayIcon(paused bool) []byte {
	name := "assets/app"
	if paused {
		name += "-paused"
	}
	if runtime.GOOS == "windows" {
		name += ".ico"
	} else {
		name += ".png"
	}
	data, _ := assets.ReadFile(name)
	return data
}

func runTray(ctx context.Context, svc *service, e *env, open func()) {
	systray.Run(func() { trayMenu(ctx, svc, e, open) }, nil)
}

func trayMenu(ctx context.Context, svc *service, e *env, open func()) {
	systray.SetIcon(trayIcon(false))
	systray.SetTitle("Скробблер Last.fm")
	systray.SetTooltip("Скробблер Last.fm")
	systray.SetOnTapped(open)

	track := systray.AddMenuItem("Ничего не играет", "")
	track.Disable()
	info := systray.AddMenuItem("", "")
	info.Disable()
	systray.AddSeparator()
	openItem := systray.AddMenuItem("Открыть окно", "")
	pauseItem := systray.AddMenuItemCheckbox("Приостановить скробблинг", "", svc.Paused())
	radioItem := systray.AddMenuItem("Радио", "")
	var autoItem *systray.MenuItem
	var autoClicked <-chan struct{}
	if autostartSupported {
		autoItem = systray.AddMenuItemCheckbox("Запускать при входе в Windows", "", autostartEnabled())
		autoClicked = autoItem.ClickedCh
	}
	profileItem := systray.AddMenuItem("Профиль на Last.fm", "")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Выход", "")

	b := &uiBackend{svc: svc, env: e}
	last := trayView{}
	update := func() {
		v := buildTrayView(b)
		if v.track != last.track {
			track.SetTitle(v.track)
			systray.SetTooltip(v.tooltip)
		}
		if v.info != last.info {
			info.SetTitle(v.info)
		}
		if v.paused != last.paused || last.track == "" {
			systray.SetIcon(trayIcon(v.paused))
			if v.paused {
				pauseItem.Check()
			} else {
				pauseItem.Uncheck()
			}
		}
		if v.loggedIn {
			profileItem.Enable()
		} else {
			profileItem.Disable()
		}
		if v.radio != last.radio {
			radioItem.SetTitle(v.radio)
		}
		if v.radioOK {
			radioItem.Enable()
		} else {
			radioItem.Disable()
		}
		if autoItem != nil {
			if autostartEnabled() {
				autoItem.Check()
			} else {
				autoItem.Uncheck()
			}
		}
		last = v
	}

	go func() {
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			update()
			select {
			case <-ctx.Done():
				systray.Quit()
				return
			case <-quitItem.ClickedCh:
				systray.Quit()
				return
			case <-openItem.ClickedCh:
				open()
			case <-pauseItem.ClickedCh:
				svc.SetPaused(!svc.Paused())
			case <-radioItem.ClickedCh:
				toggleRadio(svc, e, open)
			case <-autoClicked:
				if err := setAutostart(e, !autostartEnabled()); err != nil {
					e.log.Error("не удалось изменить автозапуск", "err", err)
				}
			case <-profileItem.ClickedCh:
				if err := b.Open("profile"); err != nil {
					e.log.Warn("не удалось открыть профиль", "err", err)
				}
			case <-tick.C:
			}
		}
	}()
}

func toggleRadio(svc *service, e *env, open func()) {
	if svc.radio.Status().Playing {
		svc.radio.Stop()
		return
	}
	st, ok := svc.radio.LastStation()
	if !ok {
		return
	}
	if err := svc.radio.Play(st.ID); err != nil {
		e.log.Warn("не удалось включить радио", "err", err)
		return
	}
	if !windowRunning() {
		open()
	}
}

type trayView struct {
	track, info, tooltip string
	paused, loggedIn     bool
	radio                string
	radioOK              bool
}

func buildTrayView(b *uiBackend) trayView {
	st := b.State()
	v := trayView{track: "Ничего не играет", paused: st.Paused, loggedIn: st.LoggedIn}
	if n := st.Now; n != nil {
		v.track = shorten(n.Artist+" — "+n.Title, 60)
	}
	switch {
	case !st.LoggedIn:
		v.info = "Вход не выполнен — откройте окно"
	case st.Paused:
		v.info = "Скробблинг приостановлен"
	case st.Now != nil && st.Now.Status == "playing":
		n := st.Now
		switch {
		case !n.Scrobblable:
			v.info = n.Player + " · слишком короткий трек"
		case n.Scrobbled:
			v.info = n.Player + " · засчитано"
		default:
			v.info = n.Player + " · засчитается через " + fmtDuration(time.Duration((n.Target-n.Played)*float64(time.Second)))
		}
	case st.Link.State == "offline":
		v.info = fmt.Sprintf("Нет связи с Last.fm · в очереди %d", st.QueueLen)
	case st.QueueLen > 0:
		v.info = fmt.Sprintf("В очереди: %d", st.QueueLen)
	default:
		v.info = fmt.Sprintf("Сегодня засчитано: %d", st.Today.Count)
	}
	v.tooltip = shorten("Скробблер Last.fm\n"+v.track, 120)

	r := st.Radio
	switch {
	case r.Playing:
		v.radio, v.radioOK = "Выключить радио «"+shorten(r.Name, 40)+"»", true
		if r.Title != "" {
			v.tooltip = shorten("Радио «"+r.Name+"»\n"+r.Title, 120)
		}
	case r.Count == 0:
		v.radio = "Радио: добавьте станцию в окне"
	default:
		name := ""
		for _, s := range b.Stations() {
			if s.ID == r.Last {
				name = s.Name
			}
		}
		if name == "" {
			if list := b.Stations(); len(list) > 0 {
				name = list[0].Name
			}
		}
		v.radio, v.radioOK = "Слушать радио «"+shorten(name, 40)+"»", true
	}
	return v
}

func shorten(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
