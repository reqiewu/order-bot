package jitter_test

import (
	"testing"
	"time"

	"github.com/reqiewu/order-bot/internal/jitter"
)

func TestAroundStaysInBand(t *testing.T) {
	base := 90 * time.Second
	for i := 0; i < 200; i++ {
		d := jitter.Around(base, 0.15)
		if d < 76*time.Second || d > 104*time.Second {
			t.Fatalf("around=%s out of ±15%% band", d)
		}
	}
}

func TestBetween(t *testing.T) {
	for i := 0; i < 100; i++ {
		d := jitter.Between(100*time.Millisecond, 400*time.Millisecond)
		if d < 100*time.Millisecond || d > 400*time.Millisecond {
			t.Fatalf("between=%s", d)
		}
	}
}
