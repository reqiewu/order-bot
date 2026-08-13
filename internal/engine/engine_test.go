package engine_test

import (
	"testing"

	"github.com/reqiewu/order-bot/internal/applog"
	"github.com/reqiewu/order-bot/internal/catalog"
	"github.com/reqiewu/order-bot/internal/engine"
	"github.com/reqiewu/order-bot/internal/marketport"
	"github.com/reqiewu/order-bot/internal/money"
	"github.com/reqiewu/order-bot/internal/spread"
)

func TestDedupBadToGood(t *testing.T) {
	d := engine.NewMemoryDeduper()
	buy := money.FromTONFloat(8)
	if d.ShouldAlert("mrkt", "1", buy, false) {
		t.Fatal("bad should not alert")
	}
	d.Remember("mrkt", "1", buy, false)
	if !d.ShouldAlert("mrkt", "1", buy, true) {
		t.Fatal("bad→good should alert")
	}
	d.Remember("mrkt", "1", buy, true)
	if d.ShouldAlert("mrkt", "1", buy, true) {
		t.Fatal("already good should skip")
	}
}

func TestDedupNewGood(t *testing.T) {
	d := engine.NewMemoryDeduper()
	if !d.ShouldAlert("portals", "x", 1, true) {
		t.Fatal("new good should alert")
	}
}

type captureAlert struct {
	n    int
	last engine.Signal
}

func (c *captureAlert) PaperSignal(s engine.Signal) error {
	c.n++
	c.last = s
	return nil
}

func salesStub(_ string, _ catalog.Lot) ([]money.NanoTON, error) {
	return []money.NanoTON{
		money.FromTONFloat(19),
		money.FromTONFloat(20),
		money.FromTONFloat(21),
		money.FromTONFloat(20),
		money.FromTONFloat(22),
	}, nil
}

func lot(id, market string, price float64) catalog.Lot {
	return catalog.Lot{
		Market:    market,
		ListingID: id,
		ModelBG: catalog.ModelBG{
			Collection: "Fine Pen",
			Model:      "Detective",
			Backdrop:   "Turquoise",
		},
		Price: money.FromTONFloat(price),
		URL:   "https://example/" + market + "/" + id,
	}
}

func TestQuotePicksHighestAskIncludingGetgems(t *testing.T) {
	cap := &captureAlert{}
	eng := engine.New(applog.Nop(), spread.DefaultFees(), salesStub, cap, engine.NewMemoryDeduper())
	key := "w"
	eng.Handle(engine.MarketEvent{
		Market: marketport.MarketGetgems, WatchID: key,
		Listings: []catalog.Lot{lot("g1", marketport.MarketGetgems, 20)},
	})
	eng.Handle(engine.MarketEvent{
		Market: marketport.MarketPortals, WatchID: key,
		Listings: []catalog.Lot{lot("p1", marketport.MarketPortals, 18)},
	})
	eng.Handle(engine.MarketEvent{
		Market: marketport.MarketMRKT, WatchID: key,
		Listings: []catalog.Lot{lot("m1", marketport.MarketMRKT, 8)},
	})
	if cap.n != 1 {
		t.Fatalf("alerts=%d", cap.n)
	}
	if cap.last.BuyMarket != marketport.MarketMRKT || cap.last.SellMarket != marketport.MarketGetgems {
		t.Fatalf("buy=%s sell=%s", cap.last.BuyMarket, cap.last.SellMarket)
	}
	if cap.last.SellURL != "https://example/getgems/g1" {
		t.Fatalf("sell url=%s", cap.last.SellURL)
	}
}

func TestGetgemsIsBuyVenue(t *testing.T) {
	cap := &captureAlert{}
	eng := engine.New(applog.Nop(), spread.DefaultFees(), salesStub, cap, engine.NewMemoryDeduper())
	eng.Handle(engine.MarketEvent{
		Market: marketport.MarketMRKT, WatchID: "w",
		Listings: []catalog.Lot{lot("m1", marketport.MarketMRKT, 20)},
	})
	eng.Handle(engine.MarketEvent{
		Market: marketport.MarketGetgems, WatchID: "w",
		Listings: []catalog.Lot{lot("g1", marketport.MarketGetgems, 8)},
	})
	if cap.n != 1 {
		t.Fatalf("alerts=%d", cap.n)
	}
	if cap.last.BuyMarket != marketport.MarketGetgems || cap.last.SellMarket != marketport.MarketMRKT {
		t.Fatalf("buy=%s sell=%s", cap.last.BuyMarket, cap.last.SellMarket)
	}
}

func TestQuotePicksHighestAskIncludingTonnel(t *testing.T) {
	cap := &captureAlert{}
	eng := engine.New(applog.Nop(), spread.DefaultFees(), salesStub, cap, engine.NewMemoryDeduper())
	key := "w"
	eng.Handle(engine.MarketEvent{
		Market: marketport.MarketTonnel, WatchID: key,
		Listings: []catalog.Lot{lot("t1", marketport.MarketTonnel, 22)},
	})
	eng.Handle(engine.MarketEvent{
		Market: marketport.MarketGetgems, WatchID: key,
		Listings: []catalog.Lot{lot("g1", marketport.MarketGetgems, 20)},
	})
	eng.Handle(engine.MarketEvent{
		Market: marketport.MarketMRKT, WatchID: key,
		Listings: []catalog.Lot{lot("m1", marketport.MarketMRKT, 8)},
	})
	if cap.n != 1 {
		t.Fatalf("alerts=%d", cap.n)
	}
	if cap.last.BuyMarket != marketport.MarketMRKT || cap.last.SellMarket != marketport.MarketTonnel {
		t.Fatalf("buy=%s sell=%s", cap.last.BuyMarket, cap.last.SellMarket)
	}
	if cap.last.SellURL != "https://example/tonnel/t1" {
		t.Fatalf("sell url=%s", cap.last.SellURL)
	}
}

func TestTonnelIsBuyVenue(t *testing.T) {
	cap := &captureAlert{}
	eng := engine.New(applog.Nop(), spread.DefaultFees(), salesStub, cap, engine.NewMemoryDeduper())
	eng.Handle(engine.MarketEvent{
		Market: marketport.MarketMRKT, WatchID: "w",
		Listings: []catalog.Lot{lot("m1", marketport.MarketMRKT, 20)},
	})
	eng.Handle(engine.MarketEvent{
		Market: marketport.MarketTonnel, WatchID: "w",
		Listings: []catalog.Lot{lot("t1", marketport.MarketTonnel, 8)},
	})
	if cap.n != 1 {
		t.Fatalf("alerts=%d", cap.n)
	}
	if cap.last.BuyMarket != marketport.MarketTonnel || cap.last.SellMarket != marketport.MarketMRKT {
		t.Fatalf("buy=%s sell=%s", cap.last.BuyMarket, cap.last.SellMarket)
	}
}
