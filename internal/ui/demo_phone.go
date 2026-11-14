package ui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func demoToken() string {
	buf := make([]byte, 16)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}

func (d *Demo) History(limit int) []Scrobble {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	at := func(daysAgo, h, m int) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day()-daysAgo, h, m, 0, 0, time.Local)
	}
	list := append([]Scrobble(nil), d.recent...)
	more := []Scrobble{
		{ID: 1280, Started: at(0, 18, 27), Artist: "Charli XCX", Title: "Von dutch", Album: "BRAT", Status: "sent"},
		{ID: 1279, Started: at(0, 18, 24), Artist: "Charli XCX", Title: "Apple", Album: "BRAT", Status: "sent"},
		{Started: at(0, 13, 5), Artist: "Кино", Title: "Группа крови", Album: "Группа крови", Status: "lastfm"},
		{Started: at(0, 13, 1), Artist: "Земфира", Title: "Искала", Album: "Прости меня моя любовь", Status: "lastfm"},
		{ID: 1276, Started: at(1, 23, 48), Artist: "Lana Del Rey", Title: "Video Games", Album: "Born to Die", Status: "sent"},
		{ID: 1275, Started: at(1, 23, 43), Artist: "Ninajirachi", Title: "All I Am", Album: "I Love My Computer", Status: "sent"},
		{ID: 1274, Started: at(1, 23, 40), Artist: "MGK", Title: "my ex's best friend", Album: "Tickets to My Downfall", Status: "sent"},
		{ID: 1273, Started: at(1, 21, 12), Artist: "Сплин", Title: "Выхода нет", Album: "Гранатовый альбом", Status: "sent"},
	}
	list = d.local(append(list, more...))
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list
}

func (d *Demo) Phone() PhoneInfo {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.phoneInfoLocked()
}

func (d *Demo) phoneInfoLocked() PhoneInfo {
	info := PhoneInfo{Enabled: d.phone != nil}
	if d.phone != nil {
		info.URL = fmt.Sprintf("http://127.0.0.1:%d%s", d.phone.Port(), d.phone.Path())
	}
	return info
}

func (d *Demo) SetPhone(enabled bool) (PhoneInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if enabled && d.phone == nil {
		if err := d.openPhoneLocked(); err != nil {
			return d.phoneInfoLocked(), err
		}
	}
	if !enabled && d.phone != nil {
		d.phone.Close()
		d.phone = nil
	}
	return d.phoneInfoLocked(), nil
}

func (d *Demo) ResetPhone() (PhoneInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.phoneToken = demoToken()
	if d.phone != nil {
		d.phone.Close()
		d.phone = nil
		if err := d.openPhoneLocked(); err != nil {
			return d.phoneInfoLocked(), err
		}
	}
	return d.phoneInfoLocked(), nil
}

func (d *Demo) openPhoneLocked() error {
	if d.phoneToken == "" {
		d.phoneToken = demoToken()
	}
	p, err := ListenPhone(d, "127.0.0.1:0", d.phoneToken, nil)
	if err != nil {
		return err
	}
	d.phone = p
	go p.Serve()
	return nil
}
