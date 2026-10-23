package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/player"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/queue"
)

func cmdStatus(ctx context.Context, e *env, args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "Настройки:\t%s\n", e.cfgPath)
	fmt.Fprintf(w, "Сессия:\t%s\n", e.sessionPath)
	fmt.Fprintf(w, "Очередь:\t%s\n", e.queuePath)

	fmt.Fprintf(w, "Ключи API:\t%s\n", yesNo(e.cfg.APIKey != "" && e.cfg.APISecret != ""))
	if e.session.Key != "" {
		fmt.Fprintf(w, "Аккаунт:\t%s (вход %s)\n",
			e.session.Username, e.session.CreatedAt.Local().Format("2006-01-02 15:04"))
	} else {
		fmt.Fprintf(w, "Аккаунт:\tне авторизован — выполните «lastfm-scrobbler auth»\n")
	}

	fmt.Fprintf(w, "Порог скроббла:\t%.0f%% длительности или %s\n",
		e.cfg.ScrobblePercent*100, e.cfg.MaxPlayTime.Duration())
	fmt.Fprintf(w, "Мин. длина трека:\t%s\n", e.cfg.MinTrackLength.Duration())
	fmt.Fprintf(w, "Источник данных:\t%s\n", player.Source)
	fmt.Fprintf(w, "Опрос плееров:\t%s\n", e.cfg.PollInterval.Duration())
	if len(e.cfg.AllowedPlayers) > 0 {
		fmt.Fprintf(w, "Только плееры:\t%v\n", e.cfg.AllowedPlayers)
	}
	if len(e.cfg.IgnoredPlayers) > 0 {
		fmt.Fprintf(w, "Игнорируются:\t%v\n", e.cfg.IgnoredPlayers)
	}

	q, err := queue.Open(e.queuePath, e.cfg.QueueLimit)
	switch {
	case err != nil && q == nil:
		fmt.Fprintf(w, "В очереди:\tошибка чтения: %v\n", err)
	case err != nil:
		fmt.Fprintf(w, "В очереди:\t%d (предупреждение: %v)\n", q.Len(), err)
	default:
		fmt.Fprintf(w, "В очереди:\t%d\n", q.Len())
	}
	for _, line := range platformStatus() {
		fmt.Fprintf(w, "%s:\t%s\n", line[0], line[1])
	}
	w.Flush()

	fmt.Println()
	fmt.Println("Плееры, видимые сейчас:")
	players, err := player.List(ctx)
	if err != nil {
		fmt.Printf("  недоступны: %v\n", err)
		return nil
	}
	if len(players) == 0 {
		fmt.Println("  плееры не найдены — запустите воспроизведение и повторите")
		return nil
	}

	pw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(pw, "  ПЛЕЕР\tСОСТОЯНИЕ\tПОЗИЦИЯ\tТРЕК")
	for _, p := range players {
		length := ""
		if p.Track.Length > 0 {
			length = "/" + fmtDuration(p.Track.Length)
		}
		fmt.Fprintf(pw, "  %s\t%s\t%s%s\t%s\n",
			p.Name, statusText(p.Status), fmtDuration(p.Position), length, p.Track.String())
	}
	pw.Flush()
	return nil
}

func statusText(s media.Status) string {
	switch s {
	case media.Playing:
		return "играет"
	case media.Paused:
		return "пауза"
	case media.Stopped:
		return "остановлен"
	default:
		return "неизвестно"
	}
}

func fmtDuration(d time.Duration) string {
	if d <= 0 {
		return "0:00"
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int(d % time.Hour / time.Minute)
	s := int(d % time.Minute / time.Second)
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func yesNo(b bool) string {
	if b {
		return "заданы"
	}
	return "не заданы"
}
