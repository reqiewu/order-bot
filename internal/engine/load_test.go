package engine_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/reqiewu/order-bot/internal/applog"
	"github.com/reqiewu/order-bot/internal/catalog"
	"github.com/reqiewu/order-bot/internal/engine"
	"github.com/reqiewu/order-bot/internal/marketport"
	"github.com/reqiewu/order-bot/internal/money"
	"github.com/reqiewu/order-bot/internal/spread"
)

type nopAlert struct{ n int }

func (a *nopAlert) PaperSignal(engine.Signal) error { a.n++; return nil }

func makeLots(market, model string, n int, price float64) []catalog.Lot {
	out := make([]catalog.Lot, n)
	for i := 0; i < n; i++ {
		num := i + 1
		out[i] = catalog.Lot{
			Market:    market,
			ListingID: fmt.Sprintf("%s-%d", market, i),
			ModelBG: catalog.ModelBG{
				Collection: "Fine Pen",
				Model:      model,
				Backdrop:   "Turquoise",
			},
			Price:  money.FromTONFloat(price),
			Number: &num,
			URL:    "https://example/" + market,
		}
	}
	return out
}

func TestLoadBookHandle(t *testing.T) {
	sizes := []int{50, 200, 500, 1000}
	fmt.Println()
	fmt.Println("=== engine Handle (3 books, same MODEL_BG, no spread) ===")
	fmt.Printf("%8s %12s %10s\n", "lots", "handle_ms", "alerts")
	for _, n := range sizes {
		cap := &nopAlert{}
		eng := engine.New(applog.Nop(), spread.DefaultFees(), salesStub, cap, engine.NewMemoryDeduper())
		eng.Handle(engine.MarketEvent{Market: marketport.MarketGetgems, WatchID: "w", Listings: makeLots("getgems", "Detective", n, 10)})
		eng.Handle(engine.MarketEvent{Market: marketport.MarketPortals, WatchID: "w", Listings: makeLots("portals", "Detective", n, 10)})
		start := time.Now()
		eng.Handle(engine.MarketEvent{Market: marketport.MarketMRKT, WatchID: "w", Listings: makeLots("mrkt", "Detective", n, 10)})
		elapsed := time.Since(start)
		fmt.Printf("%8d %12.1f %10d\n", n, elapsed.Seconds()*1000, cap.n)
	}

	fmt.Println()
	fmt.Println("=== engine Handle WITH spread (buy 7 vs ask 11, sales stub) ===")
	fmt.Printf("%8s %12s %10s\n", "lots", "handle_ms", "alerts")
	for _, n := range []int{50, 200, 500} {
		cap := &nopAlert{}
		eng := engine.New(applog.Nop(), spread.DefaultFees(), salesStub, cap, engine.NewMemoryDeduper())
		eng.Handle(engine.MarketEvent{Market: marketport.MarketGetgems, WatchID: "w", Listings: makeLots("getgems", "Detective", n, 11)})
		eng.Handle(engine.MarketEvent{Market: marketport.MarketPortals, WatchID: "w", Listings: makeLots("portals", "Detective", n, 11)})
		start := time.Now()
		eng.Handle(engine.MarketEvent{Market: marketport.MarketMRKT, WatchID: "w", Listings: makeLots("mrkt", "Detective", n, 7)})
		fmt.Printf("%8d %12.1f %10d\n", n, time.Since(start).Seconds()*1000, cap.n)
	}
}

func TestLoadAPIBudget(t *testing.T) {
	// Mid-point gate: min + jitter/2. Workers parallel; slots sequential per market.
	type mkt struct {
		name     string
		httpMs   float64
		pageSize int
		extraReq float64 // collections resolve / second on-sale path
	}
	markets := []mkt{
		{name: "mrkt", httpMs: 275, pageSize: 20, extraReq: 0},
		{name: "portals", httpMs: 300, pageSize: 20, extraReq: 1},  // collection catalog once
		{name: "getgems", httpMs: 500, pageSize: 100, extraReq: 2}, // collections + empty on-chain page
	}
	const (
		pollSec   = 90.0
		headroom  = 0.70 // 30% запас на 429 / jitter −15%
		slotGapMs = 325.0
		budgetSec = pollSec * headroom
	)
	listingNs := []int{20, 50, 100, 200, 500, 1000, 2000, 5000}

	fmt.Println()
	fmt.Println("=== max WatchSlots that fit in 63s (70% of 90s poll), one collection-size per slot ===")
	fmt.Printf("%10s", "listings")
	for _, m := range markets {
		fmt.Printf(" %12s", m.name)
	}
	fmt.Println()
	for _, n := range listingNs {
		fmt.Printf("%10d", n)
		for _, m := range markets {
			pages := (n + m.pageSize - 1) / m.pageSize
			if pages < 1 {
				pages = 1
			}
			req := float64(pages) + m.extraReq
			perSlot := req*m.httpMs + slotGapMs
			maxSlots := int(budgetSec * 1000 / perSlot)
			if maxSlots < 1 {
				maxSlots = 0
			}
			fmt.Printf(" %12d", maxSlots)
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("=== seconds for N slots × listings (should stay < 63s) ===")
	fmt.Printf("%6s %8s %10s %10s %10s\n", "slots", "listings", "mrkt_s", "portals_s", "getgems_s")
	for _, slots := range []int{1, 3, 5, 8, 12, 20} {
		for _, n := range []int{50, 200, 500, 1000} {
			fmt.Printf("%6d %8d", slots, n)
			for _, m := range markets {
				pages := (n + m.pageSize - 1) / m.pageSize
				if pages < 1 {
					pages = 1
				}
				req := float64(pages) + m.extraReq
				sec := (float64(slots)*req*m.httpMs + float64(max(slots-1, 0))*slotGapMs) / 1000
				fmt.Printf(" %10.1f", sec)
			}
			fmt.Println()
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
