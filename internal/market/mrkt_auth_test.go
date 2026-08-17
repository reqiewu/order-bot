package market_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/reqiewu/order-bot/internal/market"
)

func TestExchangeMRKTAuthURL(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var got map[string]string
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("body: %v", err)
		}
		if got["data"] != "user=1&hash=abc" {
			t.Errorf("data=%q", got["data"])
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"})
	}))
	t.Cleanup(srv.Close)

	tok, err := market.ExchangeMRKTAuthURL(context.Background(), srv.URL, "tma user=1&hash=abc")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("token=%q", tok)
	}
}
