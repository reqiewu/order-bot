package market_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/reqiewu/order-bot/internal/market"
	"github.com/reqiewu/order-bot/internal/marketport"
)

func TestGetgemsListAndSales(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/gifts/collections", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "test-key" {
			http.Error(w, "no auth", 401)
			return
		}
		writeGetgems(w, map[string]any{
			"cursor": "",
			"items": []map[string]any{
				{"address": "EQ-finepen", "name": "Fine Pen"},
			},
		})
	})
	mux.HandleFunc("/nfts/offchain/on-sale/EQ-finepen", func(w http.ResponseWriter, r *http.Request) {
		writeGetgems(w, map[string]any{
			"cursor": "",
			"items": []map[string]any{
				{
					"address":        "EQ-lot-1",
					"name":           "Fine Pen #12",
					"collectionName": "Fine Pen",
					"attributes": []map[string]any{
						{"traitType": "Model", "value": "Detective"},
						{"traitType": "Backdrop", "value": "Turquoise"},
					},
					"sale": map[string]any{
						"type": "FixPriceSale", "fullPrice": "8500000000", "currency": "TON",
					},
				},
				{
					"address": "EQ-auction",
					"name":    "Fine Pen #1",
					"sale":    map[string]any{"type": "Auction", "currency": "TON"},
				},
			},
		})
	})
	mux.HandleFunc("/nfts/on-sale/EQ-finepen", func(w http.ResponseWriter, r *http.Request) {
		writeGetgems(w, map[string]any{"cursor": "", "items": []any{}})
	})
	mux.HandleFunc("/nfts/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		writeGetgems(w, map[string]any{"items": []any{}})
	})
	mux.HandleFunc("/collection/history/EQ-finepen", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("types") != "sold" {
			t.Errorf("types=%q", r.URL.Query().Get("types"))
		}
		writeGetgems(w, map[string]any{
			"cursor": "",
			"items": []map[string]any{
				{
					"address": "EQ-sold",
					"name":    "Fine Pen #9",
					"time":    "2026-08-13T10:00:00.000Z",
					"typeData": map[string]any{
						"type": "sold", "priceNano": "10000000000", "currency": "TON",
					},
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := market.NewGetgems(market.GetgemsConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		HTTP:    srv.Client(),
	})
	listings, err := client.List(context.Background(), marketport.WatchItem{
		Collection: "Fine Pen",
		Model:      "Detective",
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
	if !strings.Contains(got.URL, "getgems.io/nft/EQ-lot-1") {
		t.Fatalf("url=%s", got.URL)
	}

	sales, err := client.RecentSales(context.Background(), marketport.Listing{
		Collection: "Fine Pen", Model: "Detective", Backdrop: "Turquoise",
	}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sales) != 1 || sales[0].Price != 10 {
		t.Fatalf("sales=%+v", sales)
	}
}

func TestGetgemsSalesHydrateFiltersModel(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/gifts/collections", func(w http.ResponseWriter, r *http.Request) {
		writeGetgems(w, map[string]any{
			"cursor": "",
			"items":  []map[string]any{{"address": "EQ-finepen", "name": "Fine Pen"}},
		})
	})
	mux.HandleFunc("/collection/history/EQ-finepen", func(w http.ResponseWriter, r *http.Request) {
		writeGetgems(w, map[string]any{
			"cursor": "",
			"items": []map[string]any{
				{
					"address": "EQ-match",
					"name":    "Fine Pen #9",
					"time":    "2026-08-13T10:00:00.000Z",
					"typeData": map[string]any{
						"type": "sold", "priceNano": "10000000000", "currency": "TON",
					},
				},
				{
					"address": "EQ-other",
					"name":    "Fine Pen #8",
					"time":    "2026-08-13T11:00:00.000Z",
					"typeData": map[string]any{
						"type": "sold", "priceNano": "11000000000", "currency": "TON",
					},
				},
			},
		})
	})
	mux.HandleFunc("/nfts/list", func(w http.ResponseWriter, r *http.Request) {
		writeGetgems(w, map[string]any{
			"items": []map[string]any{
				{
					"address": "EQ-match",
					"attributes": []map[string]any{
						{"traitType": "Model", "value": "Detective"},
						{"traitType": "Backdrop", "value": "Turquoise"},
					},
				},
				{
					"address": "EQ-other",
					"attributes": []map[string]any{
						{"traitType": "Model", "value": "Noir"},
					},
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := market.NewGetgems(market.GetgemsConfig{
		BaseURL: srv.URL,
		APIKey:  "k",
		HTTP:    srv.Client(),
	})
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

func TestGetgemsListCachesCollectionAcrossModels(t *testing.T) {
	t.Parallel()
	var offchainHits int
	mux := http.NewServeMux()
	mux.HandleFunc("/gifts/collections", func(w http.ResponseWriter, r *http.Request) {
		writeGetgems(w, map[string]any{
			"cursor": "",
			"items":  []map[string]any{{"address": "EQ-finepen", "name": "Fine Pen"}},
		})
	})
	mux.HandleFunc("/nfts/offchain/on-sale/EQ-finepen", func(w http.ResponseWriter, r *http.Request) {
		offchainHits++
		writeGetgems(w, map[string]any{
			"cursor": "",
			"items": []map[string]any{
				{
					"address":        "EQ-det",
					"name":           "Fine Pen #1",
					"collectionName": "Fine Pen",
					"attributes":     []map[string]any{{"traitType": "Model", "value": "Detective"}},
					"sale":           map[string]any{"type": "FixPriceSale", "fullPrice": "7000000000", "currency": "TON"},
				},
				{
					"address":        "EQ-noir",
					"name":           "Fine Pen #2",
					"collectionName": "Fine Pen",
					"attributes":     []map[string]any{{"traitType": "Model", "value": "Noir"}},
					"sale":           map[string]any{"type": "FixPriceSale", "fullPrice": "8000000000", "currency": "TON"},
				},
			},
		})
	})
	mux.HandleFunc("/nfts/on-sale/EQ-finepen", func(w http.ResponseWriter, r *http.Request) {
		writeGetgems(w, map[string]any{"cursor": "", "items": []any{}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := market.NewGetgems(market.GetgemsConfig{BaseURL: srv.URL, APIKey: "k", HTTP: srv.Client()})

	det, err := client.List(context.Background(), marketport.WatchItem{Collection: "Fine Pen", Model: "Detective"})
	if err != nil {
		t.Fatal(err)
	}
	noir, err := client.List(context.Background(), marketport.WatchItem{Collection: "Fine Pen", Model: "Noir"})
	if err != nil {
		t.Fatal(err)
	}
	if len(det) != 1 || det[0].Model != "Detective" {
		t.Fatalf("det=%+v", det)
	}
	if len(noir) != 1 || noir[0].Model != "Noir" {
		t.Fatalf("noir=%+v", noir)
	}
	if offchainHits != 1 {
		t.Fatalf("offchain hits=%d, want 1 (collection cache)", offchainHits)
	}
}

func writeGetgems(w http.ResponseWriter, response any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "response": response})
}
