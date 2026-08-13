package fx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/reqiewu/order-bot/internal/fx"
)

func TestCMCRate(t *testing.T) {
	t.Parallel()
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":11419,"quote":[{"symbol":"USD","price":1.3378}]}]}`))
	}))
	t.Cleanup(srv.Close)

	c := &fx.CMC{
		HTTP:    srv.Client(),
		BaseURL: srv.URL,
		CoinID:  11419,
		TTL:     time.Minute,
	}
	q1, err := c.Rate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if q1.USDPerTON < 1.33 || q1.USDPerTON > 1.34 {
		t.Fatalf("price=%v", q1.USDPerTON)
	}
	if q1.Source != "CMC" {
		t.Fatalf("source=%q", q1.Source)
	}
	q2, err := c.Rate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected cache hit, fetches=%d", n)
	}
	if q2.USDPerTON != q1.USDPerTON {
		t.Fatalf("cached mismatch")
	}
}
