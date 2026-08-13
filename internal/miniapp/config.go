package miniapp

import (
	"fmt"
	"os"
	"strings"
)

// Config — Mini App HTTP.
type Config struct {
	Addr        string
	StaticDir   string
	BotToken    string
	OperatorID  int64
	DevBypassID int64
	DevMode     bool
}

func ConfigFromEnv(botToken string, operatorID int64) (Config, error) {
	addr := strings.TrimSpace(os.Getenv("MINIAPP_ADDR"))
	if addr == "" {
		addr = ":8080"
	}
	if strings.EqualFold(os.Getenv("MINIAPP_DISABLED"), "1") ||
		strings.EqualFold(os.Getenv("MINIAPP_DISABLED"), "true") {
		addr = ""
	}
	static := strings.TrimSpace(os.Getenv("MINIAPP_STATIC_DIR"))
	if static == "" {
		static = "web/dist"
	}
	cfg := Config{
		Addr:       addr,
		StaticDir:  static,
		BotToken:   botToken,
		OperatorID: operatorID,
	}
	if v := strings.TrimSpace(os.Getenv("MINIAPP_DEV")); v == "1" || strings.EqualFold(v, "true") {
		cfg.DevMode = true
	}
	if raw := strings.TrimSpace(os.Getenv("MINIAPP_DEV_USER_ID")); raw != "" && cfg.DevMode {
		var id int64
		if _, err := fmt.Sscan(raw, &id); err != nil || id <= 0 {
			return Config{}, fmt.Errorf("miniapp: MINIAPP_DEV_USER_ID must be positive int")
		}
		cfg.DevBypassID = id
	}
	return cfg, nil
}

func (c Config) Enabled() bool { return c.Addr != "" }
