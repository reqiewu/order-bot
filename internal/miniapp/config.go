package miniapp

import (
	"fmt"
	"net"
	"net/url"
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
	CORSOrigin  string // empty = no CORS (same-origin only)
}

func ConfigFromEnv(botToken string, operatorID int64) (Config, error) {
	addr := listenAddrFromEnv()
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
	if pub := strings.TrimSpace(os.Getenv("MINIAPP_PUBLIC_URL")); pub != "" {
		u, err := url.Parse(pub)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return Config{}, fmt.Errorf("miniapp: MINIAPP_PUBLIC_URL invalid")
		}
		cfg.CORSOrigin = u.Scheme + "://" + u.Host
	}
	if cfg.Addr == "" {
		return cfg, nil
	}
	if operatorID == 0 {
		return Config{}, fmt.Errorf("miniapp: OPERATOR_TELEGRAM_ID required when Mini App is enabled")
	}
	if cfg.DevMode && !listenIsLoopback(cfg.Addr) {
		return Config{}, fmt.Errorf("miniapp: MINIAPP_DEV only allowed on loopback listen addr (got %q)", cfg.Addr)
	}
	return cfg, nil
}

func (c Config) Enabled() bool { return c.Addr != "" }

// listenAddrFromEnv — MINIAPP_PORT (8080 или :8080). Alias: MINIAPP_ADDR.
func listenAddrFromEnv() string {
	raw := strings.TrimSpace(os.Getenv("MINIAPP_PORT"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("MINIAPP_ADDR"))
	}
	if raw == "" {
		return ":8080"
	}
	if strings.Contains(raw, ":") {
		return raw
	}
	return ":" + raw
}

// listenIsLoopback is true only when the listen host is explicitly loopback.
// Bare ":8080" / "0.0.0.0:8080" bind all interfaces → false.
func listenIsLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// ":8080" is invalid SplitHostPort in older Go? Actually ":8080" works with host="".
		if strings.HasPrefix(addr, ":") {
			return false
		}
		return false
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
