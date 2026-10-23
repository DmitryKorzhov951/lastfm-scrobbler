package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
)

func cmdConfig(e *env, args []string) error {
	action := "show"
	if len(args) > 0 {
		action = args[0]
	}

	switch action {
	case "path":
		fmt.Println(e.cfgPath)
		return nil

	case "show":
		data, err := json.MarshalIndent(e.cfg, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil

	case "init":
		return configInit(e)

	default:
		return fmt.Errorf("неизвестное действие %q (доступны: path, init, show)", action)
	}
}

func configInit(e *env) error {
	if _, err := os.Stat(e.cfgPath); err == nil {
		fmt.Printf("Файл настроек уже существует: %s\n", e.cfgPath)
		fmt.Print("Перезаписать ключи API? [y/N]: ")
		if !askYes() {
			return nil
		}
	}

	fmt.Println("Создайте приложение на https://www.last.fm/api/account/create")
	fmt.Println("(поля Application name — любое, Callback URL можно оставить пустым)")
	fmt.Println()

	cfg := e.cfg
	if v := prompt("API key", cfg.APIKey); v != "" {
		cfg.APIKey = v
	}
	if v := prompt("Shared secret", cfg.APISecret); v != "" {
		cfg.APISecret = v
	}
	if cfg.APIKey == "" || cfg.APISecret == "" {
		return fmt.Errorf("нужны оба значения: api_key и api_secret")
	}

	if err := config.Save(e.cfgPath, cfg); err != nil {
		return err
	}
	fmt.Printf("\nНастройки сохранены: %s (права 0600)\n", e.cfgPath)
	fmt.Println("Дальше: lastfm-scrobbler auth")
	return nil
}

func prompt(label, current string) string {
	if current != "" {
		fmt.Printf("%s [%s]: ", label, mask(current))
	} else {
		fmt.Printf("%s: ", label)
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return ""
	}
	return strings.TrimSpace(line)
}

func askYes() bool {
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes" || answer == "д" || answer == "да"
}

func mask(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}
