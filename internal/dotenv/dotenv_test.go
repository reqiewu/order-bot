package dotenv_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/reqiewu/order-bot/internal/dotenv"
)

func TestLoadKeepsAmpersand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "PORTALS_TMA=user=1&auth_date=2&hash=abc\nMRKT_TOKEN=uuid-here\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Unsetenv("PORTALS_TMA")
	_ = os.Unsetenv("MRKT_TOKEN")
	if err := dotenv.Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("PORTALS_TMA"); got != "user=1&auth_date=2&hash=abc" {
		t.Fatalf("PORTALS_TMA=%q", got)
	}
	if got := os.Getenv("MRKT_TOKEN"); got != "uuid-here" {
		t.Fatalf("MRKT_TOKEN=%q", got)
	}
}

func TestLoadDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("FOO=fromfile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FOO", "fromenv")
	if err := dotenv.Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("FOO"); got != "fromenv" {
		t.Fatalf("got %q", got)
	}
}
