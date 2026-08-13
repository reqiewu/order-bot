package market_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/reqiewu/order-bot/internal/market"
	"github.com/reqiewu/order-bot/internal/marketport"
)

func TestTonnelListAndSales(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/pageGifts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		var req struct {
			Page   int    `json:"page"`
			Limit  int    `json:"limit"`
			Sort   string `json:"sort"`
			Filter string `json:"filter"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode pageGifts: %v", err)
			http.Error(w, "bad json", 400)
			return
		}
		if req.Page != 1 || req.Limit <= 0 {
			t.Errorf("page=%d limit=%d", req.Page, req.Limit)
		}
		if !strings.Contains(req.Sort, `"price":1`) {
			t.Errorf("sort=%s", req.Sort)
		}
		var filter map[string]any
		if err := json.Unmarshal([]byte(req.Filter), &filter); err != nil {
			t.Errorf("filter json: %v %s", err, req.Filter)
		}
		if filter["gift_name"] != "Fine Pen" || filter["modelName"] != "Detective" {
			t.Errorf("filter=%s", req.Filter)
		}
		if filter["asset"] != "TON" {
			t.Errorf("asset=%v", filter["asset"])
		}
		writeJSON(w, []map[string]any{
			{
				"gift_id":  4242,
				"gift_num": 12,
				"name":     "Fine Pen",
				"model":    "Detective (2%)",
				"backdrop": "Turquoise (1.5%)",
				"symbol":   "Star (0.5%)",
				"price":    8.5,
				"asset":    "TON",
				"status":   "forsale",
			},
			{
				"gift_id":    7,
				"gift_num":   1,
				"name":       "Fine Pen",
				"model":      "Detective (2%)",
				"price":      9,
				"asset":      "TON",
				"status":     "auction",
				"auction_id": "auc-1",
			},
			{
				"gift_id":          8,
				"name":             "Fine Pen",
				"model":            "Detective (2%)",
				"price":            3,
				"asset":            "TON",
				"status":           "forsale",
				"dutchAuctionData": map[string]any{"startAt": "2026-01-01"},
			},
		})
	})
	mux.HandleFunc("/api/saleHistory", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			AuthData string         `json:"authData"`
			Type     string         `json:"type"`
			Filter   map[string]any `json:"filter"`
			Sort     map[string]any `json:"sort"`
		}
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Errorf("decode saleHistory: %v", err)
			http.Error(w, "bad json", 400)
			return
		}
		if req.AuthData != "init-data" {
			http.Error(w, `{"status":"error","message":"Invalid auth data"}`, 200)
			return
		}
		if req.Type != "SALE" {
			t.Errorf("type=%s", req.Type)
		}
		if req.Filter["gift_name"] != "Fine Pen" {
			t.Errorf("filter=%s", raw)
		}
		if req.Sort["timestamp"] != float64(-1) {
			t.Errorf("sort=%v", req.Sort)
		}
		writeJSON(w, []map[string]any{
			{
				"gift_name": "Fine Pen",
				"gift_num":  9,
				"model":     "Detective (2%)",
				"backdrop":  "Turquoise (1.5%)",
				"price":     10,
				"asset":     "TON",
				"type":      "SALE",
				"timestamp": "2026-08-13T10:00:00.000Z",
			},
			{
				"gift_name": "Fine Pen",
				"model":     "Detective (2%)",
				"price":     99,
				"type":      "BID",
				"timestamp": "2026-08-13T11:00:00.000Z",
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := market.NewTonnel(market.TonnelConfig{
		BaseURL:  srv.URL,
		InitData: "init-data",
		HTTP:     srv.Client(),
	})
	listings, err := client.List(context.Background(), marketport.WatchItem{
		Collection: "Fine Pen",
		Model:      "Detective",
		Backdrop:   "Turquoise",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 1 {
		t.Fatalf("listings=%d %+v", len(listings), listings)
	}
	got := listings[0]
	if got.Price != 8.5 || got.Model != "Detective" || got.Backdrop != "Turquoise" {
		t.Fatalf("%+v", got)
	}
	if got.Number == nil || *got.Number != 12 {
		t.Fatalf("number=%v", got.Number)
	}
	if got.ID != "4242" || !strings.Contains(got.URL, "/nft/4242") {
		t.Fatalf("id=%s url=%s", got.ID, got.URL)
	}
	if got.Market != marketport.MarketTonnel {
		t.Fatalf("market=%s", got.Market)
	}

	sales, err := client.RecentSales(context.Background(), marketport.Listing{
		Collection: "Fine Pen", Model: "Detective", Backdrop: "Turquoise",
	}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sales) != 1 || sales[0].Price != 10 || sales[0].Model != "Detective" {
		t.Fatalf("sales=%+v", sales)
	}
}

func TestTonnelSalesNeedAuth(t *testing.T) {
	t.Parallel()
	client := market.NewTonnel(market.TonnelConfig{BaseURL: "http://127.0.0.1:1"})
	_, err := client.RecentSales(context.Background(), marketport.Listing{Collection: "Fine Pen"}, 5)
	if err == nil {
		t.Fatal("expected error without initData")
	}
}

func TestTonnelSalesRejectInvalidAuth(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/saleHistory", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"error","message":"Invalid auth data"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := market.NewTonnel(market.TonnelConfig{
		BaseURL:  srv.URL,
		InitData: "bad",
		HTTP:     srv.Client(),
	})
	_, err := client.RecentSales(context.Background(), marketport.Listing{Collection: "Fine Pen"}, 5)
	if err == nil || !market.IsUnauthorized(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestTonnelListPaginatesUntilShortPage(t *testing.T) {
	t.Parallel()
	var pages []int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/pageGifts", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Page  int `json:"page"`
			Limit int `json:"limit"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		pages = append(pages, req.Page)
		item := func(id int) map[string]any {
			return map[string]any{
				"gift_id": id, "gift_num": id, "name": "Fine Pen",
				"model": "Detective (2%)", "price": 5, "asset": "TON", "status": "forsale",
			}
		}
		if req.Page == 1 {
			writeJSON(w, []map[string]any{item(1), item(2)})
			return
		}
		writeJSON(w, []map[string]any{item(3)})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := market.NewTonnel(market.TonnelConfig{
		BaseURL:  srv.URL,
		HTTP:     srv.Client(),
		PageSize: 2,
	})
	listings, err := client.List(context.Background(), marketport.WatchItem{Collection: "Fine Pen", Model: "Detective"})
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 3 {
		t.Fatalf("listings=%d", len(listings))
	}
	if len(pages) != 2 || pages[0] != 1 || pages[1] != 2 {
		t.Fatalf("pages=%v", pages)
	}
}

func TestTonnelChromeDoerHitsHTTPTest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/pageGifts", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]any{
			{
				"gift_id": 1, "gift_num": 1, "name": "Fine Pen",
				"model": "Detective (2%)", "price": 5, "asset": "TON", "status": "forsale",
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := market.NewTonnel(market.TonnelConfig{BaseURL: srv.URL, PageSize: 30})
	listings, err := client.List(context.Background(), marketport.WatchItem{Collection: "Fine Pen", Model: "Detective"})
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 1 || listings[0].Price != 5 {
		t.Fatalf("listings=%+v", listings)
	}
}

func TestTonnelLivePageGifts(t *testing.T) {
	if os.Getenv("TONNEL_LIVE") != "1" {
		t.Skip("set TONNEL_LIVE=1 to hit gifts3.tonnel.network")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := market.ProbeTonnel(ctx, ""); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
