package market_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reqiewu/order-bot/internal/marketport"
	"github.com/reqiewu/order-bot/internal/market"
)

func TestStaticToken(t *testing.T) {
	t.Parallel()

	tok, err := market.StaticToken("abc").Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok != "abc" {
		t.Fatalf("Token = %q, want abc", tok)
	}

	_, err = market.StaticToken("").Token(context.Background())
	if err == nil {
		t.Fatal("expected error for empty static token")
	}
}

func TestStaticTokenFromEnv(t *testing.T) {
	t.Setenv("MRKT_TOKEN", "from-env")
	auth := market.StaticTokenFromEnv("MRKT_TOKEN")
	tok, err := auth.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok != "from-env" {
		t.Fatalf("Token = %q, want from-env", tok)
	}
}

func TestMRKTListNormalizesAndPaginates(t *testing.T) {
	t.Parallel()

	page1 := readTestdata(t, "saling_page1.json")
	page2 := readTestdata(t, "saling_page2.json")

	var gotAuth string
	var pages int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/gifts/saling" {
			http.NotFound(w, r)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		if got := r.Header.Get("Cookie"); !strings.Contains(got, "access_token=test-token") {
			t.Errorf("Cookie = %q, want access_token", got)
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)

		pages++
		cursor, _ := req["cursor"].(string)
		switch cursor {
		case "", "null":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(page1)
		case "page-2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(page2)
		default:
			t.Errorf("unexpected cursor %q", cursor)
			http.Error(w, "bad cursor", 500)
		}
	}))
	t.Cleanup(srv.Close)

	client := market.NewMRKT(market.Config{
		BaseURL:  srv.URL,
		Auth:     market.StaticToken("test-token"),
		HTTP:     srv.Client(),
		MaxPages: 5,
	})

	listings, err := client.List(context.Background(), marketport.WatchItem{
		Collection: "Lunar Snake",
		Model:      "Albino",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotAuth != "test-token" {
		t.Fatalf("Authorization = %q, want test-token", gotAuth)
	}
	if pages != 2 {
		t.Fatalf("pages fetched = %d, want 2", pages)
	}
	if len(listings) != 3 {
		t.Fatalf("listings = %d, want 3: %+v", len(listings), listings)
	}

	byID := map[string]marketport.Listing{}
	for _, l := range listings {
		byID[l.ID] = l
	}
	cheap := byID["lot-cheap"]
	if cheap.Collection != "Lunar Snake" || cheap.Model != "Albino" || cheap.Price != 90 {
		t.Fatalf("cheap listing mismatch: %+v", cheap)
	}
	if cheap.Number == nil || *cheap.Number != 42 {
		t.Fatalf("cheap.Number = %v, want 42", cheap.Number)
	}
	if cheap.Backdrop != "Black" || cheap.Symbol != "Star" {
		t.Fatalf("cheap meta = %+v", cheap)
	}
	if cheap.URL != "https://t.me/mrkt/lot/cheap" {
		t.Fatalf("cheap.URL = %q", cheap.URL)
	}
	if byID["lot-mid"].URL == "" {
		t.Fatal("expected fallback URL when API omits url")
	}
}

func TestMRKTListConvertsNanoSalePrice(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"gifts":[{
				"id":"peach-981",
				"collectionName":"Precious Peach",
				"modelName":"Rosegold",
				"backdropName":"Pacific Green",
				"symbolName":"Feather",
				"number":981,
				"salePrice":263149800000
			}],
			"cursor":""
		}`))
	}))
	t.Cleanup(srv.Close)

	client := market.NewMRKT(market.Config{
		BaseURL: srv.URL,
		Auth:    market.StaticToken("t"),
		HTTP:    srv.Client(),
	})
	listings, err := client.List(context.Background(), marketport.WatchItem{
		Collection: "Precious Peach",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("listings=%d", len(listings))
	}
	got := listings[0]
	if got.Price < 263.1 || got.Price > 263.2 {
		t.Fatalf("Price=%v want ~263.15 TON (not nano)", got.Price)
	}
	if got.Model != "Rosegold" || got.Backdrop != "Pacific Green" || got.Symbol != "Feather" {
		t.Fatalf("meta=%+v want Rosegold / Pacific Green / Feather", got)
	}
}

func TestMRKTListRespectsMaxPages(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"gifts":[{"id":"a","collection":"X","model":"Y","price_ton":1}],"cursor":"next"}`))
	}))
	t.Cleanup(srv.Close)

	client := market.NewMRKT(market.Config{
		BaseURL:  srv.URL,
		Auth:     market.StaticToken("t"),
		HTTP:     srv.Client(),
		MaxPages: 2,
	})
	listings, err := client.List(context.Background(), marketport.WatchItem{Collection: "X", Model: "Y"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listings) != 2 {
		t.Fatalf("want 2 listings from 2 pages, got %d", len(listings))
	}
}

func TestMRKTListSendsBackdropFilterWhenSet(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"gifts":[{"id":"lot-1","collection":"Lunar Snake","model":"Albino","backdrop":"Black","symbol":"Star","price_ton":90}],"cursor":""}`))
	}))
	t.Cleanup(srv.Close)

	client := market.NewMRKT(market.Config{
		BaseURL: srv.URL,
		Auth:    market.StaticToken("t"),
		HTTP:    srv.Client(),
	})

	listings, err := client.List(context.Background(), marketport.WatchItem{
		Collection: "Lunar Snake",
		Model:      "Albino",
		Backdrop:   "Black",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	names, _ := gotBody["backdropNames"].([]any)
	if len(names) != 1 || names[0] != "Black" {
		t.Fatalf("backdropNames = %#v, want [Black]", gotBody["backdropNames"])
	}
	if len(listings) != 1 {
		t.Fatalf("listings = %d, want 1", len(listings))
	}
	if listings[0].Backdrop != "Black" || listings[0].Symbol != "Star" {
		t.Fatalf("listing meta = %+v", listings[0])
	}
}

func TestMRKTListOmitsBackdropFilterWhenUnset(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"gifts":[],"cursor":""}`))
	}))
	t.Cleanup(srv.Close)

	client := market.NewMRKT(market.Config{
		BaseURL: srv.URL,
		Auth:    market.StaticToken("t"),
		HTTP:    srv.Client(),
	})

	_, err := client.List(context.Background(), marketport.WatchItem{
		Collection: "Lunar Snake",
		Model:      "Albino",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	names, _ := gotBody["backdropNames"].([]any)
	if len(names) != 0 {
		t.Fatalf("backdropNames = %#v, want empty slice", gotBody["backdropNames"])
	}
}

func TestMRKTListUnauthorized(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	client := market.NewMRKT(market.Config{
		BaseURL: srv.URL,
		Auth:    market.StaticToken("bad"),
		HTTP:    srv.Client(),
	})
	_, err := client.List(context.Background(), marketport.WatchItem{Collection: "Lunar Snake"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !market.IsUnauthorized(err) {
		t.Fatalf("IsUnauthorized = false, err = %v", err)
	}
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	return b
}
