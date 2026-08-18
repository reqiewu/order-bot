package config

import (
	"testing"
)

func TestFromEnvSessionAndBot(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "bot-token")
	t.Setenv("OPERATOR_TELEGRAM_ID", "42")
	t.Setenv("TELEGRAM_API_ID", "123")
	t.Setenv("TELEGRAM_API_HASH", "hash")
	t.Setenv("TELEGRAM_SESSION_PATH", "data/tg.session")
	t.Setenv("MRKT_TOKEN", "ignored")
	t.Setenv("PORTALS_TOKEN", "ignored")
	t.Setenv("GETGEMS_TOKEN", "ignored")
	t.Setenv("TONNEL_TOKEN", "ignored")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramToken != "bot-token" {
		t.Fatalf("TelegramToken=%q", cfg.TelegramToken)
	}
	if cfg.OperatorID != 42 {
		t.Fatalf("OperatorID=%d", cfg.OperatorID)
	}
	if cfg.TelegramAPIID != 123 {
		t.Fatalf("TelegramAPIID=%d", cfg.TelegramAPIID)
	}
	if cfg.TelegramAPIHash != "hash" {
		t.Fatalf("TelegramAPIHash=%q", cfg.TelegramAPIHash)
	}
	if cfg.MetricsAddr != ":9091" {
		t.Fatalf("MetricsAddr default=%q", cfg.MetricsAddr)
	}
}

func TestFromEnvMetricsAddrOff(t *testing.T) {
	t.Setenv("METRICS_ADDR", "")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MetricsAddr != "" {
		t.Fatalf("MetricsAddr=%q want empty", cfg.MetricsAddr)
	}
}
