package market_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/reqiewu/order-bot/internal/market"
)

func TestPortalsMarketConfigParsesLiveShape(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/market/config" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"commission":"0.02",
			"offer_fee":"0.01",
			"withdrawal_fee":"0.3",
			"user_cashback":"0"
		}`))
	}))
	t.Cleanup(srv.Close)

	p := market.NewPortals(market.PortalsConfig{
		BaseURL: srv.URL,
		HTTP:    srv.Client(),
	})
	cfg, err := p.MarketConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Commission != 0.02 || cfg.OfferFee != 0.01 || cfg.WithdrawalFee != 0.3 {
		t.Fatalf("%+v", cfg)
	}
}

func TestPortalsMarketConfigAcceptsNumericJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"commission":0.05,"offer_fee":0,"withdrawal_fee":1.2}`))
	}))
	t.Cleanup(srv.Close)

	p := market.NewPortals(market.PortalsConfig{BaseURL: srv.URL, HTTP: srv.Client()})
	cfg, err := p.MarketConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Commission != 0.05 || cfg.WithdrawalFee != 1.2 {
		t.Fatalf("%+v", cfg)
	}
}
