package config

import (
	"testing"
)

func TestFromEnvPrefersNewTokenNames(t *testing.T) {
	t.Setenv("PORTALS_TOKEN", "new-portals")
	t.Setenv("PORTALS_TMA", "old-portals")
	t.Setenv("GETGEMS_TOKEN", "new-getgems")
	t.Setenv("GETGEMS_API_KEY", "old-getgems")
	t.Setenv("TONNEL_TOKEN", "new-tonnel")
	t.Setenv("TONNEL_INITDATA", "old-tonnel")
	t.Setenv("MRKT_TOKEN", "mrkt")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PortalsTMA != "new-portals" {
		t.Fatalf("PortalsTMA=%q", cfg.PortalsTMA)
	}
	if cfg.GetgemsAPIKey != "new-getgems" {
		t.Fatalf("GetgemsAPIKey=%q", cfg.GetgemsAPIKey)
	}
	if cfg.TonnelInitData != "new-tonnel" {
		t.Fatalf("TonnelInitData=%q", cfg.TonnelInitData)
	}
	if cfg.MRKTToken != "mrkt" {
		t.Fatalf("MRKTToken=%q", cfg.MRKTToken)
	}
}

func TestFromEnvFallsBackToLegacyTokenNames(t *testing.T) {
	t.Setenv("PORTALS_TOKEN", "")
	t.Setenv("PORTALS_TMA", "old-portals")
	t.Setenv("GETGEMS_TOKEN", "")
	t.Setenv("GETGEMS_API_KEY", "old-getgems")
	t.Setenv("TONNEL_TOKEN", "")
	t.Setenv("TONNEL_INITDATA", "old-tonnel")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PortalsTMA != "old-portals" {
		t.Fatalf("PortalsTMA=%q", cfg.PortalsTMA)
	}
	if cfg.GetgemsAPIKey != "old-getgems" {
		t.Fatalf("GetgemsAPIKey=%q", cfg.GetgemsAPIKey)
	}
	if cfg.TonnelInitData != "old-tonnel" {
		t.Fatalf("TonnelInitData=%q", cfg.TonnelInitData)
	}
}
