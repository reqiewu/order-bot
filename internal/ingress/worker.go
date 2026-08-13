package ingress

import (
	"context"
	"log/slog"
	"time"

	"github.com/reqiewu/order-bot/internal/catalog"
	"github.com/reqiewu/order-bot/internal/engine"
	"github.com/reqiewu/order-bot/internal/marketport"
	"github.com/reqiewu/order-bot/internal/money"
)

// Reader — marketport.MarketReader + имя маркета.
type Reader struct {
	Name   string
	Client marketport.MarketReader
}

// Worker периодически снимает полный List по слотам и шлёт в Engine.
type Worker struct {
	Log      *slog.Logger
	Reader   Reader
	Slots    func() []catalog.WatchSlot
	Interval time.Duration
	Out      chan<- engine.MarketEvent
}

func (w *Worker) Run(ctx context.Context) {
	if w.Interval <= 0 {
		w.Interval = 90 * time.Second
	}
	log := w.Log
	if log == nil {
		log = slog.Default()
	}
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.tick(ctx, log)
			t.Reset(w.Interval)
		}
	}
}

func (w *Worker) tick(ctx context.Context, log *slog.Logger) {
	slots := w.Slots()
	for _, slot := range slots {
		if !slot.Valid() {
			continue
		}
		watch := marketport.WatchItem{
			Collection: slot.Collection,
			Model:      slot.Model,
			Backdrop:   slot.Backdrop,
		}
		listings, err := w.Reader.Client.List(ctx, watch)
		if err != nil {
			log.Warn("list failed", "market", w.Reader.Name, "slot", slot.ID, "err", err)
			continue
		}
		lots := make([]catalog.Lot, 0, len(listings))
		for _, l := range listings {
			lots = append(lots, catalog.Lot{
				Market:    w.Reader.Name,
				ListingID: l.ID,
				ModelBG: catalog.ModelBG{
					Collection: l.Collection,
					Model:      l.Model,
					Backdrop:   l.Backdrop,
				},
				Price:  money.FromTONFloat(l.Price),
				Number: l.Number,
				URL:    l.URL,
			})
		}
		ev := engine.MarketEvent{
			Market:    w.Reader.Name,
			WatchID:   slot.ID,
			Listings:  lots,
			FetchedAt: time.Now().UTC(),
		}
		select {
		case <-ctx.Done():
			return
		case w.Out <- ev:
		}
	}
}

// SalesOnDemand реализует engine.SalesFunc.
func SalesOnDemand(readers map[string]marketport.MarketReader, limit int, maxAge time.Duration) engine.SalesFunc {
	if limit <= 0 {
		limit = marketport.DefaultSaleLimit
	}
	if maxAge <= 0 {
		maxAge = 72 * time.Hour
	}
	return func(sellMarket string, like catalog.Lot) ([]money.NanoTON, error) {
		r := readers[sellMarket]
		if r == nil {
			return nil, nil
		}
		src := marketport.Listing{
			ID:         like.ListingID,
			Collection: like.ModelBG.Collection,
			Model:      like.ModelBG.Model,
			Backdrop:   like.ModelBG.Backdrop,
			Price:      float64(like.Price) / float64(money.TON),
			Number:     like.Number,
			URL:        like.URL,
		}
		sales, err := r.RecentSales(context.Background(), src, limit)
		if err != nil {
			return nil, err
		}
		cutoff := time.Now().Add(-maxAge)
		var out []money.NanoTON
		for _, s := range sales {
			if !s.At.IsZero() && s.At.Before(cutoff) {
				continue
			}
			out = append(out, money.FromTONFloat(s.Price))
		}
		return out, nil
	}
}
