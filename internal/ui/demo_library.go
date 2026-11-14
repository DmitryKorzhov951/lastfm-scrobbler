package ui

import (
	"math/rand"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/library"
)

type demoAlbum struct {
	artist, album string
	tracks        []string
	weight        int
}

var demoCatalog = []demoAlbum{
	{"Charli XCX", "BRAT", []string{"360", "Von dutch", "Apple", "Sympathy is a knife", "Guess"}, 30},
	{"Lana Del Rey", "Born to Die", []string{"Summertime Sadness", "Video Games", "Born to Die"}, 22},
	{"Lana Del Rey", "Ultraviolence", []string{"West Coast", "Brooklyn Baby"}, 10},
	{"MGK", "Tickets to My Downfall", []string{"bloody valentine", "my ex's best friend"}, 14},
	{"Ninajirachi", "I Love My Computer", []string{"iPod Touch", "All I Am"}, 12},
	{"100 gecs", "1000 gecs and the Tree of Clues", []string{"ringtone (remix) [feat. Charli XCX, Rico Nasty, Kero Kero Bonito]"}, 9},
	{"Кино", "Группа крови", []string{"Группа крови", "Война", "Спокойная ночь"}, 16},
	{"Земфира", "Прости меня моя любовь", []string{"Хочешь?", "Искала"}, 11},
	{"Сплин", "Гранатовый альбом", []string{"Выхода нет", "Орбит без сахара"}, 8},
}

func demoLibrary(now time.Time) *library.Store {
	lib, _ := library.Open("", "")
	r := rand.New(rand.NewSource(7))
	total := 0
	for _, a := range demoCatalog {
		total += a.weight
	}
	pick := func() demoAlbum {
		n := r.Intn(total)
		for _, a := range demoCatalog {
			if n < a.weight {
				return a
			}
			n -= a.weight
		}
		return demoCatalog[0]
	}
	var plays []library.Play
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -400)
	for ; day.Before(now); day = day.AddDate(0, 0, 1) {
		at := day.Add(time.Duration(9+r.Intn(10))*time.Hour + time.Duration(r.Intn(60))*time.Minute)
		for n := 8 + r.Intn(25); n > 0 && at.Before(now.Add(-time.Hour)); n-- {
			a := pick()
			t := a.tracks[r.Intn(len(a.tracks))]
			plays = append(plays, library.Play{At: at, Artist: a.artist, Title: t, Album: a.album})
			at = at.Add(time.Duration(150+r.Intn(150)) * time.Second)
		}
	}
	lib.Add(plays)
	return lib
}
