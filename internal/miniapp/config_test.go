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
