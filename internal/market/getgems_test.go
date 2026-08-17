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
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		op := r.URL.Query().Get("operationName")
		vars := map[string]any{}
		_ = json.Unmarshal([]byte(r.URL.Query().Get("variables")), &vars)
		switch op {
		case "collectionSearch":
			writeGQL(w, map[string]any{
				"alphaNftCollectionSearch": map[string]any{
					"edges": []map[string]any{
						{"node": map[string]any{"address": "EQ-finepen", "name": "Fine Pen"}},
					},
					"info": map[string]any{"hasNextPage": false},
				},
			})
		case "getNftCollectionByAddress":
			writeGQL(w, map[string]any{
				"nftCollectionByAddress": map[string]any{
					"address": vars["address"], "name": "Fine Pen", "type": "TgGifts",
				},
			})
		case "nftSearchInstantSell":
			writeGQL(w, map[string]any{
				"alphaNftItemSearch": map[string]any{
					"edges": []map[string]any{
						gqlLot("EQ-lot-1", "Fine Pen #12", "Detective", "Turquoise", "8500000000"),
						gqlAuction("EQ-auction", "Fine Pen #1"),
					},
					"info": map[string]any{"hasNextPage": false},
				},
			})
		case "historyCollectionNftItems":
			got, _ := json.Marshal(vars["types"])
			if string(got) != `["Sold"]` {
				t.Errorf("types=%s", got)
			}
			writeGQL(w, map[string]any{
				"historyCollectionNftItems": map[string]any{
					"cursor": "",
					"items": []map[string]any{
						gqlSold("EQ-sold", "Fine Pen #9", "10000000000", 1786894980),
					},
				},
			})
		case "getNftByAddress":
			writeGQL(w, map[string]any{
				"nft": map[string]any{
					"address": vars["address"],
					"name":    "Fine Pen #9",
					"attributes": []map[string]any{
						{"traitType": "Model", "value": "Detective"},
						{"traitType": "Backdrop", "value": "Turquoise"},
					},
				},
			})
		default:
			http.Error(w, "unknown op "+op, 400)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := market.NewGetgems(market.GetgemsConfig{BaseURL: srv.URL, HTTP: srv.Client()})
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
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		op := r.URL.Query().Get("operationName")
		vars := map[string]any{}
		_ = json.Unmarshal([]byte(r.URL.Query().Get("variables")), &vars)
		switch op {
		case "collectionSearch":
			writeGQL(w, map[string]any{
				"alphaNftCollectionSearch": map[string]any{
					"edges": []map[string]any{
						{"node": map[string]any{"address": "EQ-finepen", "name": "Fine Pen"}},
					},
				},
			})
		case "getNftCollectionByAddress":
			writeGQL(w, map[string]any{
				"nftCollectionByAddress": map[string]any{"address": vars["address"], "name": "Fine Pen", "type": "TgGifts"},
			})
		case "historyCollectionNftItems":
			writeGQL(w, map[string]any{
				"historyCollectionNftItems": map[string]any{
					"cursor": "",
					"items": []map[string]any{
						gqlSold("EQ-match", "Fine Pen #9", "10000000000", 1786894980),
						gqlSold("EQ-other", "Fine Pen #8", "11000000000", 1786891578),
					},
				},
			})
		case "getNftByAddress":
			addr, _ := vars["address"].(string)
			model := "Noir"
			backdrop := ""
			if addr == "EQ-match" {
				model, backdrop = "Detective", "Turquoise"
			}
			attrs := []map[string]any{{"traitType": "Model", "value": model}}
			if backdrop != "" {
				attrs = append(attrs, map[string]any{"traitType": "Backdrop", "value": backdrop})
			}
			writeGQL(w, map[string]any{
				"nft": map[string]any{"address": addr, "name": "Fine Pen", "attributes": attrs},
			})
		default:
			http.Error(w, "unknown op "+op, 400)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := market.NewGetgems(market.GetgemsConfig{BaseURL: srv.URL, HTTP: srv.Client()})
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
	var searchHits, colSearchHits int
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		op := r.URL.Query().Get("operationName")
		vars := map[string]any{}
		_ = json.Unmarshal([]byte(r.URL.Query().Get("variables")), &vars)
		switch op {
		case "collectionSearch":
			colSearchHits++
			writeGQL(w, map[string]any{
				"alphaNftCollectionSearch": map[string]any{
					"edges": []map[string]any{
						{"node": map[string]any{"address": "EQ-finepen", "name": "Fine Pen"}},
					},
				},
			})
		case "getNftCollectionByAddress":
			writeGQL(w, map[string]any{
				"nftCollectionByAddress": map[string]any{"address": vars["address"], "name": "Fine Pen", "type": "TgGifts"},
			})
		case "nftSearchInstantSell":
			searchHits++
			writeGQL(w, map[string]any{
				"alphaNftItemSearch": map[string]any{
					"edges": []map[string]any{
						gqlLot("EQ-det", "Fine Pen #1", "Detective", "", "7000000000"),
						gqlLot("EQ-noir", "Fine Pen #2", "Noir", "", "8000000000"),
					},
					"info": map[string]any{"hasNextPage": false},
				},
			})
		default:
			http.Error(w, "unknown op "+op, 400)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := market.NewGetgems(market.GetgemsConfig{BaseURL: srv.URL, HTTP: srv.Client()})

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
	if colSearchHits != 1 {
		t.Fatalf("collectionSearch hits=%d, want 1", colSearchHits)
	}
	if searchHits != 2 {
		t.Fatalf("nft search hits=%d, want 2 (per model)", searchHits)
	}
}

func TestGetgemsResolvesPluralGiftName(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		op := r.URL.Query().Get("operationName")
		vars := map[string]any{}
		_ = json.Unmarshal([]byte(r.URL.Query().Get("variables")), &vars)
		switch op {
		case "collectionSearch":
			writeGQL(w, map[string]any{
				"alphaNftCollectionSearch": map[string]any{
					"edges": []map[string]any{
						{"cursor": "1", "node": map[string]any{"address": "EQ-gifts", "name": "Ice Creams"}},
					},
					"info": map[string]any{"hasNextPage": false},
				},
			})
		case "nftSearchInstantSell":
			q, _ := vars["query"].(string)
			if !strings.Contains(q, "EQ-gifts") {
				t.Errorf("search query=%s", q)
			}
			writeGQL(w, map[string]any{
				"alphaNftItemSearch": map[string]any{
					"edges": []map[string]any{
						gqlLot("EQ-lot", "Ice Cream #1", "Starship", "Amber", "3660000000"),
					},
					"info": map[string]any{"hasNextPage": false},
				},
			})
		default:
			http.Error(w, "unknown op "+op, 400)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := market.NewGetgems(market.GetgemsConfig{BaseURL: srv.URL, HTTP: srv.Client()})
	got, err := client.List(context.Background(), marketport.WatchItem{Collection: "Ice Cream", Model: "Starship"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Model != "Starship" {
		t.Fatalf("%+v", got)
	}
}

func TestGetgemsResolvesDurovApostrophePlural(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		op := r.URL.Query().Get("operationName")
		vars := map[string]any{}
		_ = json.Unmarshal([]byte(r.URL.Query().Get("variables")), &vars)
		switch op {
		case "collectionSearch":
			writeGQL(w, map[string]any{
				"alphaNftCollectionSearch": map[string]any{
					"edges": []map[string]any{
						{"cursor": "1", "node": map[string]any{"address": "EQ-caps", "name": "Durov’s Caps"}},
					},
					"info": map[string]any{"hasNextPage": false},
				},
			})
		case "nftSearchInstantSell":
			q, _ := vars["query"].(string)
			if !strings.Contains(q, "EQ-caps") {
				t.Errorf("search query=%s", q)
			}
			writeGQL(w, map[string]any{
				"alphaNftItemSearch": map[string]any{
					"edges": []map[string]any{
						gqlLot("EQ-lot", "Durov’s Cap #1", "Black", "", "500000000000"),
					},
					"info": map[string]any{"hasNextPage": false},
				},
			})
		default:
			http.Error(w, "unknown op "+op, 400)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := market.NewGetgems(market.GetgemsConfig{BaseURL: srv.URL, HTTP: srv.Client()})
	got, err := client.List(context.Background(), marketport.WatchItem{Collection: "Durov's Cap"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("%+v", got)
	}
}

func TestGetgemsUnknownCollectionReturnsEmpty(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("operationName") != "collectionSearch" {
			http.Error(w, "unexpected", 400)
			return
		}
		writeGQL(w, map[string]any{
			"alphaNftCollectionSearch": map[string]any{
				"edges": []map[string]any{
					{"cursor": "1", "node": map[string]any{"address": "EQ-ice", "name": "Ice Creams"}},
				},
				"info": map[string]any{"hasNextPage": false},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := market.NewGetgems(market.GetgemsConfig{BaseURL: srv.URL, HTTP: srv.Client()})
	got, err := client.List(context.Background(), marketport.WatchItem{Collection: "Fine Pen"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %+v", got)
	}
}

func writeGQL(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func gqlLot(addr, name, model, backdrop, nano string) map[string]any {
	attrs := []map[string]any{{"traitType": "Model", "value": model}}
	if backdrop != "" {
		attrs = append(attrs, map[string]any{"traitType": "Backdrop", "value": backdrop})
	}
	coll := name
	if i := strings.LastIndex(name, " #"); i > 0 {
		coll = name[:i]
	}
	return map[string]any{
		"cursor": "1",
		"node": map[string]any{
			"name":       name,
			"address":    addr,
			"index":      0,
			"attributes": attrs,
			"collection": map[string]any{"address": "EQ-col", "name": coll, "type": "TgGifts"},
			"sale": map[string]any{
				"__typename": "NftSaleFixPrice",
				"fullPrice":  nano,
				"currency":   "TON",
			},
		},
	}
}

func gqlAuction(addr, name string) map[string]any {
	return map[string]any{
		"cursor": "2",
		"node": map[string]any{
			"name":    name,
			"address": addr,
			"sale":    map[string]any{"__typename": "NftSaleAuction", "currency": "TON"},
		},
	}
}

func gqlSold(addr, name, nano string, unix int64) map[string]any {
	return map[string]any{
		"address": addr,
		"time":    unix,
		"nft":     map[string]any{"name": name, "address": addr},
		"typeData": map[string]any{
			"__typename": "HistoryTypeSold",
			"type":       "sold",
			"price":      nano,
			"currency":   "TON",
		},
	}
}
