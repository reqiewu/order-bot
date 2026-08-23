package miniapp

import "testing"

func TestListenAddrFromEnv(t *testing.T) {
	t.Setenv("MINIAPP_ADDR", "")
	t.Setenv("MINIAPP_PORT", "8081")
	if got := listenAddrFromEnv(); got != ":8081" {
		t.Fatalf("port number: %q", got)
	}
	t.Setenv("MINIAPP_PORT", ":9090")
	if got := listenAddrFromEnv(); got != ":9090" {
		t.Fatalf("port with colon: %q", got)
	}
	t.Setenv("MINIAPP_PORT", "")
	t.Setenv("MINIAPP_ADDR", ":7070")
	if got := listenAddrFromEnv(); got != ":7070" {
		t.Fatalf("legacy addr: %q", got)
	}
}

func TestListenIsLoopback(t *testing.T) {
	t.Parallel()
	if listenIsLoopback(":8080") {
		t.Fatal(":8080 must not be loopback")
	}
	if listenIsLoopback("0.0.0.0:8080") {
		t.Fatal("0.0.0.0 must not be loopback")
	}
	if !listenIsLoopback("127.0.0.1:8080") {
		t.Fatal("127.0.0.1 should be loopback")
	}
	if !listenIsLoopback("localhost:8080") {
		t.Fatal("localhost should be loopback")
	}
}

func TestConfigRequiresOperator(t *testing.T) {
	t.Setenv("MINIAPP_DISABLED", "")
	t.Setenv("MINIAPP_PORT", "8080")
	t.Setenv("MINIAPP_DEV", "")
	t.Setenv("MINIAPP_PUBLIC_URL", "")
	_, err := ConfigFromEnv("token", 0)
	if err == nil {
		t.Fatal("want operator required")
	}
	cfg, err := ConfigFromEnv("token", 42)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled() || cfg.OperatorID != 42 {
		t.Fatalf("%+v", cfg)
	}
}

func TestConfigDevModeRejectedOnPublicListen(t *testing.T) {
	t.Setenv("MINIAPP_PORT", "8080")
	t.Setenv("MINIAPP_DEV", "1")
	t.Setenv("MINIAPP_DEV_USER_ID", "42")
	t.Setenv("MINIAPP_PUBLIC_URL", "")
	_, err := ConfigFromEnv("token", 42)
	if err == nil {
		t.Fatal("want dev mode rejected on :8080")
	}
	t.Setenv("MINIAPP_PORT", "127.0.0.1:8080")
	cfg, err := ConfigFromEnv("token", 42)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DevMode || cfg.DevBypassID != 42 {
		t.Fatalf("%+v", cfg)
	}
}

func TestConfigCORSFromPublicURL(t *testing.T) {
	t.Setenv("MINIAPP_PORT", "8080")
	t.Setenv("MINIAPP_DEV", "")
	t.Setenv("MINIAPP_PUBLIC_URL", "https://xxxx.ngrok-free.app/app")
	cfg, err := ConfigFromEnv("token", 7)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CORSOrigin != "https://xxxx.ngrok-free.app" {
		t.Fatalf("CORSOrigin=%q", cfg.CORSOrigin)
	}
}
