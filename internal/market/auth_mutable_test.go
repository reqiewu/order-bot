package market_test

import (
	"context"
	"sync"
	"testing"

	"github.com/reqiewu/order-bot/internal/market"
)

func TestMutableTokenSetAndGet(t *testing.T) {
	t.Parallel()
	m := market.NewMutableToken(`" tma user=1 "`)
	tok, err := m.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tok != "tma user=1" {
		t.Fatalf("tok=%q", tok)
	}
	m.Set("user=2&hash=x")
	if m.Get() != "user=2&hash=x" {
		t.Fatalf("Get=%q", m.Get())
	}
}

func TestMutableTokenEmpty(t *testing.T) {
	t.Parallel()
	m := market.NewMutableToken("")
	if _, err := m.Token(context.Background()); err == nil {
		t.Fatal("want error for empty token")
	}
}

func TestMutableTokenConcurrent(t *testing.T) {
	t.Parallel()
	m := market.NewMutableToken("a")
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			m.Set("b")
		}()
		go func() {
			defer wg.Done()
			_, _ = m.Token(context.Background())
			_ = m.Get()
		}()
	}
	wg.Wait()
}
