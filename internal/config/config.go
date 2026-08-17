package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TelegramToken   string
	OperatorID      int64
	TonnelBaseURL   string
	TonnelDisabled  bool
	TelegramAPIID   int
	TelegramAPIHash string
	TelegramSession string
	TelegramUserOff bool // TELEGRAM_USER_DISABLED
	BoltPath        string
	PollInterval    time.Duration
	WatchJSON       string // optional bootstrap slots JSON array
	LogLevel        string // debug|info|warn|error
}

func FromEnv() (Config, error) {
	cfg := Config{
		TelegramToken:   strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TonnelBaseURL:   strings.TrimSpace(os.Getenv("TONNEL_BASE_URL")),
		TonnelDisabled:  envTruthy("TONNEL_DISABLED"),
		TelegramAPIHash: strings.TrimSpace(os.Getenv("TELEGRAM_API_HASH")),
		TelegramSession: envOr("TELEGRAM_SESSION_PATH", "data/tg.session"),
		TelegramUserOff: envTruthy("TELEGRAM_USER_DISABLED"),
		BoltPath:        envOr("BOLT_PATH", "data/order-bot.db"),
		WatchJSON:       strings.TrimSpace(os.Getenv("WATCH_SLOTS_JSON")),
		LogLevel:        logLevelFromEnv(),
	}
	if v := strings.TrimSpace(os.Getenv("TELEGRAM_API_ID")); v != "" {
		id, err := strconv.Atoi(v)
		if err != nil || id <= 0 {
			return cfg, fmt.Errorf("TELEGRAM_API_ID invalid")
		}
		cfg.TelegramAPIID = id
	}
	if v := strings.TrimSpace(os.Getenv("OPERATOR_TELEGRAM_ID")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return cfg, fmt.Errorf("OPERATOR_TELEGRAM_ID: %w", err)
		}
		cfg.OperatorID = id
	}
	sec := 1
	if v := strings.TrimSpace(os.Getenv("POLL_INTERVAL_SEC")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return cfg, fmt.Errorf("POLL_INTERVAL_SEC invalid (min 1)")
		}
		sec = n
	}
	cfg.PollInterval = time.Duration(sec) * time.Second
	return cfg, nil
}

func envTruthy(k string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
	return v == "1" || v == "true" || v == "yes"
}

func envOr(k, def string) string {
	if v := envFirst(k); v != "" {
		return v
	}
	return def
}

// envFirst — первое непустое значение; несколько ключей = новое имя, потом старый alias.
func envFirst(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// LOG_LEVEL=debug|info|warn|error; LOG_DEBUG=1 — alias для debug.
func logLevelFromEnv() string {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))); v != "" {
		switch v {
		case "debug", "info", "warn", "warning", "error":
			if v == "warning" {
				return "warn"
			}
			return v
		}
	}
	if os.Getenv("LOG_DEBUG") == "1" || strings.EqualFold(os.Getenv("LOG_DEBUG"), "true") {
		return "debug"
	}
	return "info"
}
