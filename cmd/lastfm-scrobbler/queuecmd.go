package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/history"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/queue"
)

func cmdQueue(ctx context.Context, e *env, args []string) error {
	action := "list"
	if len(args) > 0 {
		action = args[0]
	}

	q, err := queue.Open(e.queuePath, e.cfg.QueueLimit)
	if err != nil && q == nil {
		return err
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Предупреждение:", err)
	}
	var hist *history.Store
	if histPath, err := config.HistoryPath(); err == nil {
		if hist, err = history.Open(histPath, 0); err != nil {
			fmt.Fprintln(os.Stderr, "Предупреждение:", err)
		}
	}

	switch action {
	case "list":
		items := q.Items()
		if len(items) == 0 {
			fmt.Println("Очередь пуста.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ВРЕМЯ\tПОПЫТОК\tТРЕК")
		for _, it := range items {
			fmt.Fprintf(w, "%s\t%d\t%s\n",
				it.Scrobble.Timestamp.Local().Format("2006-01-02 15:04:05"),
				it.Attempts, it.Scrobble.Track.String())
		}
		w.Flush()
		fmt.Printf("\nВсего: %d\n", len(items))
		return nil

	case "flush":
		if q.Len() == 0 {
			fmt.Println("Очередь пуста.")
			return nil
		}
		client, err := e.client()
		if err != nil {
			return err
		}
		if client.SessionKey == "" {
			return fmt.Errorf("нет ключа сессии — выполните «lastfm-scrobbler auth»")
		}
		sub := newSubmitter(client, q, hist, e.cfg.RetryInterval.Duration(), false, e.log)
		before := q.Len()
		sub.flush(ctx, true)
		fmt.Printf("Отправлено: %d, осталось в очереди: %d\n", before-q.Len(), q.Len())
		return nil

	case "clear":
		ids := itemIDs(q.Items())
		n, err := q.Remove(ids...)
		if err != nil {
			return err
		}
		if hist != nil {
			if err := hist.Delete(ids...); err != nil {
				return err
			}
		}
		fmt.Printf("Удалено записей: %d\n", n)
		return nil

	default:
		return fmt.Errorf("неизвестное действие %q (доступны: list, flush, clear)", action)
	}
}
