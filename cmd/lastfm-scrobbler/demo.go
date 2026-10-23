package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/ui"
)

func cmdDemo(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:0", "адрес сервера окна")
	loggedOut := fs.Bool("logged-out", false, "показать экран входа")
	offline := fs.Bool("offline", false, "показать очередь без связи с Last.fm")
	noWindow := fs.Bool("no-window", false, "не открывать окно, только напечатать адрес")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "lastfm-scrobbler demo [-logged-out] [-offline] [-no-window] [-addr 127.0.0.1:0]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	srv, err := ui.Listen(ui.NewDemo(!*loggedOut, *offline), *addr, nil)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	fmt.Println(srv.URL())
	if !*noWindow {
		if err := openWindow(srv.URL()); err != nil {
			fmt.Fprintln(os.Stderr, "Не удалось открыть окно:", err)
		}
	}
	return srv.Serve()
}
