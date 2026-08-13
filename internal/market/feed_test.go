package market_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/reqiewu/order-bot/internal/marketport"
	"github.com/reqiewu/order-bot/internal/market"
)

func TestRecentSalesFiltersModelBackdropAndCapsAtThree(t *testing.T) {
	t.Parallel()

	page1, err := os.ReadFile(filepath.Join("testdata", "feed_page1.json"))
	if err != nil {
		t.Fatal(err)
	}
	page2, err := os.ReadFile(filepath.Join("testdata", "feed_page2.json"))
	if err != nil {
		t.Fatal(err)
	}

	var calls int
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/feed" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_, _ = w.Write(page1)
			return
		}
		_, _ = w.Write(page2)
	}))
	t.Cleanup(srv.Close)

	client := market.NewMRKT(market.Config{
		BaseURL: srv.URL,
		Auth:    market.StaticToken("tok"),
		HTTP:    srv.Client(),
	})

	sales, err := client.RecentSales(context.Background(), marketport.Listing{
		Collection: "Astral Shard",
		Model:      "Ether",
		Backdrop:   "Black",
	}, 3)
	if err != nil {
		t.Fatalf("RecentSales: %v", err)
	}
	if len(sales) != 3 {
		t.Fatalf("len=%d want 3: %+v", len(sales), sales)
	}
	// Feed newest-first 14.8, 13.1, 15.0 → return oldest→newest among comps
	want := []float64{15.0, 13.1, 14.8}
	wantNum := []int{303, 202, 101}
	for i, s := range sales {
		if s.Price != want[i] {
			t.Fatalf("sales[%d]=%v want %v", i, s.Price, want[i])
		}
		if s.Number == nil || *s.Number != wantNum[i] {
			t.Fatalf("sales[%d].Number=%v want %d", i, s.Number, wantNum[i])
		}
	}
	if calls != 1 {
		t.Fatalf("expected one feed page when limit filled, calls=%d", calls)
	}
	models, _ := gotBody["modelNames"].([]any)
	backs, _ := gotBody["backdropNames"].([]any)
	if len(models) != 1 || models[0] != "Ether" || len(backs) != 1 || backs[0] != "Black" {
		t.Fatalf("feed request filters = %+v", gotBody)
	}
}

func TestRecentSalesNoneWhenNoMatch(t *testing.T) {
	t.Parallel()
	page1, err := os.ReadFile(filepath.Join("testdata", "feed_page1.json"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(page1)
	}))
	t.Cleanup(srv.Close)

	client := market.NewMRKT(market.Config{
		BaseURL: srv.URL,
		Auth:    market.StaticToken("tok"),
		HTTP:    srv.Client(),
	})
	sales, err := client.RecentSales(context.Background(), marketport.Listing{
		Collection: "No Such",
		Model:      "X",
		Backdrop:   "Y",
	}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(sales) != 0 {
		t.Fatalf("want none, got %+v", sales)
	}
}
