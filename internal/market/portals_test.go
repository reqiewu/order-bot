package market_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/reqiewu/order-bot/internal/marketport"
	"github.com/reqiewu/order-bot/internal/market"
)

func TestPortalsListNormalizesSearch(t *testing.T) {
	t.Parallel()

	body := readTestdata(t, "portals_search.json")
	var gotPath, gotAuth, gotColIDs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/collections") {
			_, _ = w.Write([]byte(`{"collections":[{"id":"col-lunar","name":"Lunar Snake","short_name":"lunarsnake"}]}`))
			return
		}
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		q := r.URL.Query()
		gotColIDs = q.Get("collection_ids")
		if q.Get("status") != "listed" {
			t.Errorf("status = %q", q.Get("status"))
		}
		if q.Get("collection_ids") != "col-lunar" {
			t.Errorf("collection_ids = %q, want col-lunar", q.Get("collection_ids"))
		}
		if q.Get("filter_by_models") != "Albino" {
			t.Errorf("filter_by_models = %q", q.Get("filter_by_models"))
		}
		if !strings.Contains(q.Get("sort_by"), "price") {
			t.Errorf("sort_by = %q", q.Get("sort_by"))
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	client := market.NewPortals(market.PortalsConfig{
		BaseURL: srv.URL,
		Auth:    market.StaticToken("tma testdata"),
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
	if gotPath != "/api/nfts/search" {
		t.Fatalf("path = %q, want /api/nfts/search", gotPath)
	}
	if gotAuth != "tma testdata" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotColIDs != "col-lunar" {
		t.Fatalf("collection_ids = %q", gotColIDs)
	}
	if len(listings) != 2 {
		t.Fatalf("listings = %d, want 2: %+v", len(listings), listings)
	}
	first := listings[0]
	if first.ID != "019fc99b-8f27-75d2-a718-02f3b8291f6d" {
		t.Fatalf("id = %q", first.ID)
	}
	if first.Collection != "Lunar Snake" || first.Model != "Albino" || first.Backdrop != "Black" {
		t.Fatalf("meta = %+v", first)
	}
	if first.Price != 3 {
		t.Fatalf("price = %v, want 3", first.Price)
	}
	if first.Number == nil || *first.Number != 197233 {
		t.Fatalf("number = %v, want 197233", first.Number)
	}
	if first.Symbol != "Candle" {
		t.Fatalf("symbol = %q", first.Symbol)
	}
	if first.URL != "https://t.me/portals_market_bot/market?startapp=gift_019fc99b-8f27-75d2-a718-02f3b8291f6d" {
		t.Fatalf("URL = %q", first.URL)
	}
}

func TestPortalsListWorksWithoutAuth(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/collections") {
			_, _ = w.Write([]byte(`{"collections":[{"id":"col-x","name":"X","short_name":"x"}]}`))
			return
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected Authorization %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"a","name":"X","price":"1.5","attributes":[{"type":"model","value":"M"}]}]}`))
	}))
	t.Cleanup(srv.Close)

	client := market.NewPortals(market.PortalsConfig{BaseURL: srv.URL, HTTP: srv.Client()})
	listings, err := client.List(context.Background(), marketport.WatchItem{Collection: "X"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listings) != 1 || listings[0].Price != 1.5 {
		t.Fatalf("listings = %+v", listings)
	}
}

func TestPortalsRecentSalesRequiresAuthAndMapsBuys(t *testing.T) {
	t.Parallel()

	body := readTestdata(t, "portals_actions.json")
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/collections") {
			_, _ = w.Write([]byte(`{"collections":[{"id":"col-lunar","name":"Lunar Snake","short_name":"lunarsnake"}]}`))
			return
		}
		if r.URL.Path != "/api/market/actions/" {
			http.NotFound(w, r)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		q := r.URL.Query()
		if q.Get("action_types") != "sell" {
			t.Errorf("action_types = %q", q.Get("action_types"))
		}
		if q.Get("collection_ids") != "col-lunar" {
			t.Errorf("collection_ids = %q", q.Get("collection_ids"))
		}
		if q.Get("filter_by_models") != "Albino" {
			t.Errorf("filter_by_models = %q", q.Get("filter_by_models"))
		}
		if q.Get("filter_by_backdrops") != "Black" {
			t.Errorf("filter_by_backdrops = %q", q.Get("filter_by_backdrops"))
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	client := market.NewPortals(market.PortalsConfig{
		BaseURL: srv.URL,
		Auth:    market.StaticToken("raw-init-data"), // без префикса → tma …
		HTTP:    srv.Client(),
	})
	sales, err := client.RecentSales(context.Background(), marketport.Listing{
		Collection: "Lunar Snake",
		Model:      "Albino",
		Backdrop:   "Black",
	}, 3)
	if err != nil {
		t.Fatalf("RecentSales: %v", err)
	}
	if gotAuth != "tma raw-init-data" {
		t.Fatalf("Authorization = %q, want tma prefix", gotAuth)
	}
	// listing отфильтрован; other model отфильтрован; остаётся 2 buy → порядок старые→новые
	if len(sales) != 2 {
		t.Fatalf("sales = %d, want 2: %+v", len(sales), sales)
	}
	if sales[0].Price != 3.5 || sales[1].Price != 3.0 {
		t.Fatalf("order/prices = %+v", sales)
	}
	if sales[0].Number == nil || *sales[0].Number != 10 {
		t.Fatalf("first number = %v", sales[0].Number)
	}
}

func TestPortalsRecentSalesUnauthorized(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing auth"}`, http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	client := market.NewPortals(market.PortalsConfig{
		BaseURL: srv.URL,
		Auth:    market.StaticToken("tma bad"),
		HTTP:    srv.Client(),
	})
	_, err := client.RecentSales(context.Background(), marketport.Listing{Collection: "X"}, 3)
	if !market.IsUnauthorized(err) {
		t.Fatalf("err = %v, want UnauthorizedError", err)
	}
}

func TestPortalsDefaultBaseURL(t *testing.T) {
	t.Parallel()
	p := market.NewPortals(market.PortalsConfig{})
	// не дёргаем сеть — только проверяем, что конструктор не падает и интерфейс собран
	var _ marketport.MarketReader = p
	_ = p
}

func TestPortalsTMAFromEnv(t *testing.T) {
	t.Setenv("PORTALS_TMA", "user=1\nquery_id=x")
	auth := market.PortalsTMAFromEnv("PORTALS_TMA")
	tok, err := auth.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok != "user=1\nquery_id=x" {
		t.Fatalf("Token = %q", tok)
	}
}

func TestPortalsRetries429(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/collections") {
			_, _ = w.Write([]byte(`{"collections":[{"id":"col-x","name":"X","short_name":"x"}]}`))
			return
		}
		if n.Add(1) <= 2 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, `{"message":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"ok","name":"X","price":"2","attributes":[{"type":"model","value":"M"}]}]}`))
	}))
	t.Cleanup(srv.Close)

	client := market.NewPortals(market.PortalsConfig{
		BaseURL: srv.URL,
		HTTP:    srv.Client(),
	})
	listings, err := client.List(context.Background(), marketport.WatchItem{Collection: "X"})
	if err != nil {
		t.Fatalf("List after 429s: %v", err)
	}
	if len(listings) != 1 || listings[0].ID != "ok" {
		t.Fatalf("listings=%+v", listings)
	}
	if n.Load() < 3 {
		t.Fatalf("want ≥3 search attempts, got %d", n.Load())
	}
}

func TestPortalsListCacheHits(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/collections") {
			_, _ = w.Write([]byte(`{"collections":[{"id":"col-x","name":"X","short_name":"x"}]}`))
			return
		}
		n.Add(1)
		_, _ = w.Write([]byte(`{"results":[{"id":"a","name":"X","price":"1","attributes":[]}]}`))
	}))
	t.Cleanup(srv.Close)

	client := market.NewPortals(market.PortalsConfig{BaseURL: srv.URL, HTTP: srv.Client()})
	watch := marketport.WatchItem{Collection: "X"}
	if _, err := client.List(context.Background(), watch); err != nil {
		t.Fatal(err)
	}
	if _, err := client.List(context.Background(), watch); err != nil {
		t.Fatal(err)
	}
	if n.Load() != 1 {
		t.Fatalf("cache miss: calls=%d", n.Load())
	}
}

func TestPortalsSynthesizesStartAppWithoutTGID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/collections") {
			_, _ = w.Write([]byte(`{"collections":[{"id":"col-fp","name":"Fine Pen","short_name":"finepen"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"019fd684-9a71-7801-931a-6cd904d76ab6","name":"Fine Pen","price":"9.4","external_collection_number":6087,"attributes":[{"type":"model","value":"Astronaut"}]}]}`))
	}))
	t.Cleanup(srv.Close)

	client := market.NewPortals(market.PortalsConfig{BaseURL: srv.URL, HTTP: srv.Client()})
	listings, err := client.List(context.Background(), marketport.WatchItem{Collection: "Fine Pen"})
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 1 {
		t.Fatalf("%+v", listings)
	}
	want := "https://t.me/portals_market_bot/market?startapp=gift_019fd684-9a71-7801-931a-6cd904d76ab6"
	if listings[0].URL != want {
		t.Fatalf("URL=%q want %q", listings[0].URL, want)
	}
}

func TestPortalsFallsBackToTelegramNFTWithoutID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/collections") {
			_, _ = w.Write([]byte(`{"collections":[{"id":"col-fp","name":"Fine Pen","short_name":"finepen"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"name":"Fine Pen","tg_id":"FinePen-6087","price":"9.4","external_collection_number":6087,"attributes":[]}]}`))
	}))
	t.Cleanup(srv.Close)

	client := market.NewPortals(market.PortalsConfig{BaseURL: srv.URL, HTTP: srv.Client()})
	listings, err := client.List(context.Background(), marketport.WatchItem{Collection: "Fine Pen"})
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 1 {
		t.Fatalf("%+v", listings)
	}
	want := "https://t.me/nft/FinePen-6087"
	if listings[0].URL != want {
		t.Fatalf("URL=%q want %q", listings[0].URL, want)
	}
}
