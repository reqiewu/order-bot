package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TelegramToken string
	OperatorID    int64
	MRKTToken     string
	PortalsTMA    string
	GetgemsAPIKey string
	BoltPath      string
	PollInterval  time.Duration
	WatchJSON     string // optional bootstrap slots JSON array
	LogLevel      string // debug|info|warn|error
}

func FromEnv() (Config, error) {
	cfg := Config{
		TelegramToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		MRKTToken:     strings.TrimSpace(os.Getenv("MRKT_TOKEN")),
		PortalsTMA:    strings.TrimSpace(os.Getenv("PORTALS_TMA")),
		GetgemsAPIKey: strings.TrimSpace(os.Getenv("GETGEMS_API_KEY")),
		BoltPath:      envOr("BOLT_PATH", "data/order-bot.db"),
		WatchJSON:     strings.TrimSpace(os.Getenv("WATCH_SLOTS_JSON")),
		LogLevel:      logLevelFromEnv(),
	}
	if v := strings.TrimSpace(os.Getenv("OPERATOR_TELEGRAM_ID")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return cfg, fmt.Errorf("OPERATOR_TELEGRAM_ID: %w", err)
		}
		cfg.OperatorID = id
	}
	sec := 90
	if v := strings.TrimSpace(os.Getenv("POLL_INTERVAL_SEC")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 10 {
			return cfg, fmt.Errorf("POLL_INTERVAL_SEC invalid")
		}
		sec = n
	}
	cfg.PollInterval = time.Duration(sec) * time.Second
	return cfg, nil
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
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
