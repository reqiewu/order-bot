package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TelegramToken  string
	OperatorID     int64
	MRKTToken      string
	PortalsTMA     string
	BoltPath       string
	PollInterval   time.Duration
	WatchJSON      string // optional bootstrap slots JSON array
	LogDebug       bool
}

func FromEnv() (Config, error) {
	cfg := Config{
		TelegramToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		MRKTToken:     strings.TrimSpace(os.Getenv("MRKT_TOKEN")),
		PortalsTMA:    strings.TrimSpace(os.Getenv("PORTALS_TMA")),
		BoltPath:      envOr("BOLT_PATH", "data/order-bot.db"),
		WatchJSON:     strings.TrimSpace(os.Getenv("WATCH_SLOTS_JSON")),
		LogDebug:      os.Getenv("LOG_DEBUG") == "1" || strings.EqualFold(os.Getenv("LOG_LEVEL"), "debug"),
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
