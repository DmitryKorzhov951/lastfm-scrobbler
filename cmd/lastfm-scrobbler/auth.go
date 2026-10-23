package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
)

func cmdAuth(ctx context.Context, e *env, args []string) error {
	fs := flag.NewFlagSet("auth", flag.ContinueOnError)
	noBrowser := fs.Bool("no-browser", false, "не открывать браузер автоматически")
	force := fs.Bool("force", false, "перезаписать существующую сессию")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "lastfm-scrobbler auth [-no-browser] [-force]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if e.session.Key != "" && !*force {
		fmt.Printf("Уже выполнен вход как %s. Для смены аккаунта: lastfm-scrobbler auth -force\n",
			e.session.Username)
		return nil
	}

	client, err := e.client()
	if err != nil {
		return err
	}

	token, err := client.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("получение токена: %w", err)
	}

	authURL := client.AuthURL(token)
	fmt.Println("Откройте ссылку и разрешите доступ приложению:")
	fmt.Println()
	fmt.Println("   " + authURL)
	fmt.Println()

	if !*noBrowser {
		openBrowser(authURL)
	}

	fmt.Print("После подтверждения нажмите Enter...")
	if err := waitEnter(ctx); err != nil {
		return err
	}

	session, err := client.GetSession(ctx, token)
	if err != nil {
		return fmt.Errorf("обмен токена на сессию: %w "+
			"(убедитесь, что доступ подтверждён в браузере)", err)
	}

	saved := config.Session{
		Username:  session.Name,
		Key:       session.Key,
		CreatedAt: time.Now().UTC(),
	}
	if err := config.SaveSession(e.sessionPath, saved); err != nil {
		return fmt.Errorf("сохранение сессии: %w", err)
	}

	fmt.Printf("\nГотово: вход выполнен как %s\n", session.Name)
	fmt.Printf("Ключ сессии сохранён в %s (права 0600)\n", e.sessionPath)
	fmt.Println("Запустите демон: lastfm-scrobbler run")
	return nil
}

func openBrowser(url string) {
	if err := openURL(url); err != nil {
		fmt.Println("(не удалось открыть браузер автоматически — откройте ссылку вручную)")
	}
}

func waitEnter(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		bufio.NewReader(os.Stdin).ReadString('\n')
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
