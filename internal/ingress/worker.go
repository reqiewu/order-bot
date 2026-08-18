package ingress

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/reqiewu/order-bot/internal/applog"
	"github.com/reqiewu/order-bot/internal/catalog"
	"github.com/reqiewu/order-bot/internal/engine"
	"github.com/reqiewu/order-bot/internal/jitter"
	"github.com/reqiewu/order-bot/internal/marketport"
	"github.com/reqiewu/order-bot/internal/metrics"
	"github.com/reqiewu/order-bot/internal/money"
)

// Reader — marketport.MarketReader + имя маркета.
type Reader struct {
	Name   string
	Client marketport.MarketReader
}

// Worker периодически снимает топ-N дешёвых List по слотам и шлёт в Engine.
type Worker struct {
	Log        *applog.Logger
	Reader     Reader
	Slots      func() []catalog.WatchSlot
	Interval   time.Duration // fallback
	IntervalFn func() time.Duration
	Out        chan<- engine.MarketEvent

	mu   sync.Mutex
	prev map[string]map[string]catalog.Lot // market\0watchID → listingID → lot
}

func (w *Worker) interval() time.Duration {
	var base time.Duration
	if w.IntervalFn != nil {
		if d := w.IntervalFn(); d >= time.Second {
			base = d
		}
	}
	if base <= 0 {
		if w.Interval > 0 {
			base = w.Interval
		} else {
			base = time.Second
		}
	}
	// ±15% чтобы воркеры маркетов не били API синхронно.
	return jitter.Around(base, 0.15)
}

func (w *Worker) Run(ctx context.Context) {
	log := w.Log
	if log == nil {
		log = applog.Nop()
	}
	// Короткий стартовый джиттер (1с-poll не ждёт 3с).
	_ = jitter.Sleep(ctx, 0, 300*time.Millisecond)
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.tick(ctx, log)
			t.Reset(w.interval())
		}
	}
}

func readerEnabled(c marketport.MarketReader) bool {
	type enabler interface{ Enabled() bool }
	if e, ok := c.(enabler); ok {
		return e.Enabled()
	}
	return true
}

func (w *Worker) tick(ctx context.Context, log *applog.Logger) {
	if w.Reader.Client == nil || !readerEnabled(w.Reader.Client) {
		return
	}
	slots := w.Slots()
	total := 0
	ok := false
	for i, slot := range slots {
		if !slot.Valid() {
			continue
		}
		if i > 0 {
			// При быстром poll почти без паузы между слотами.
			if err := jitter.Sleep(ctx, 20*time.Millisecond, 80*time.Millisecond); err != nil {
				return
			}
		}
		watch := marketport.WatchItem{
			Collection: slot.Collection,
			Model:      slot.Model,
			Backdrop:   slot.Backdrop,
		}
		log.DebugTree("list start",
			applog.KV{K: "market", V: w.Reader.Name},
			applog.KV{K: "collection", V: orAny(slot.Collection)},
			applog.KV{K: "model", V: orAny(slot.Model)},
			applog.KV{K: "background", V: orAny(slot.Backdrop)},
		)
		started := time.Now()
		listings, err := w.Reader.Client.List(ctx, watch)
		metrics.ObservePoll(w.Reader.Name, time.Since(started), err)
		if err != nil {
			log.Warn("list failed",
				"market", w.Reader.Name,
				"slot", slot.ID,
				"elapsed", time.Since(started).Round(time.Millisecond).String(),
				"err", err,
			)
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
		ok = true
		total += len(lots)
		w.logListResult(log, slot, lots, time.Since(started))

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
	if ok {
		metrics.SetListings(w.Reader.Name, total)
	}
}

func (w *Worker) logListResult(log *applog.Logger, slot catalog.WatchSlot, lots []catalog.Lot, elapsed time.Duration) {
	key := w.Reader.Name + "\x00" + slot.ID
	next := make(map[string]catalog.Lot, len(lots))
	for _, lot := range lots {
		next[lot.ListingID] = lot
	}

	w.mu.Lock()
	if w.prev == nil {
		w.prev = map[string]map[string]catalog.Lot{}
	}
	prev := w.prev[key]
	w.prev[key] = next
	w.mu.Unlock()

	elapsedKV := applog.KV{K: "elapsed", V: elapsed.Round(time.Millisecond).String()}
	kvs := []applog.KV{
		{K: "market", V: w.Reader.Name},
		{K: "collection", V: orAny(slot.Collection)},
		{K: "model", V: orAny(slot.Model)},
		{K: "background", V: orAny(slot.Backdrop)},
		{K: "listings", V: formatListingsCount(len(lots))},
	}

	if prev == nil {
		// первый снимок — baseline, без флуда listing added
		kvs = append(kvs, applog.KV{K: "snapshot", V: "initial"}, elapsedKV)
		log.InfoTree("list ok", kvs...)
		return
	}

	type change struct {
		old, lot catalog.Lot
	}
	var added, removed []catalog.Lot
	var changed []change
	for id, lot := range next {
		old, ok := prev[id]
		if !ok {
			added = append(added, lot)
			continue
		}
		if old.Price != lot.Price {
			changed = append(changed, change{old: old, lot: lot})
		}
	}
	for id, lot := range prev {
		if _, ok := next[id]; !ok {
			removed = append(removed, lot)
		}
	}

	kvs = append(kvs,
		applog.KV{K: "added", V: fmt.Sprintf("%d", len(added))},
		applog.KV{K: "removed", V: fmt.Sprintf("%d", len(removed))},
		applog.KV{K: "changed", V: fmt.Sprintf("%d", len(changed))},
		elapsedKV,
	)
	log.InfoTree("list ok", kvs...)
	if !log.DebugEnabled() {
		return
	}
	for _, lot := range added {
		logListing(log, "listing added", lot, "")
	}
	for _, c := range changed {
		logListing(log, "listing changed", c.lot,
			fmt.Sprintf("%s → %s", formatTON(c.old.Price), formatTON(c.lot.Price)))
	}
	for _, lot := range removed {
		logListing(log, "listing removed", lot, "")
	}
}

func logListing(log *applog.Logger, msg string, lot catalog.Lot, priceOverride string) {
	price := formatTON(lot.Price)
	if priceOverride != "" {
		price = priceOverride
	}
	log.DebugTree(msg,
		applog.KV{K: "market", V: lot.Market},
		applog.KV{K: "collection", V: orAny(lot.ModelBG.Collection)},
		applog.KV{K: "model", V: orAny(lot.ModelBG.Model)},
		applog.KV{K: "background", V: orAny(lot.ModelBG.Backdrop)},
		applog.KV{K: "id", V: lot.ListingID},
		applog.KV{K: "price", V: price},
		applog.KV{K: "url", V: lot.URL},
	)
}

func orAny(s string) string {
	if strings.TrimSpace(s) == "" {
		return "any"
	}
	return s
}

func formatListingsCount(n int) string {
	if n == 0 {
		return "[]"
	}
	return fmt.Sprintf("%d", n)
}

func formatTON(p money.NanoTON) string {
	return fmt.Sprintf("%.4f TON", float64(p)/float64(money.TON))
}

// SalesOnDemand реализует engine.SalesFunc.
func SalesOnDemand(readers map[string]marketport.MarketReader, limit int, maxAge time.Duration) engine.SalesFunc {
	if limit <= 0 {
		limit = marketport.DefaultSaleLimit
	}
	if maxAge <= 0 {
		maxAge = 10 * 24 * time.Hour
	}
	return func(sellMarket string, like catalog.Lot) ([]money.NanoTON, error) {
		r := readers[sellMarket]
		if r == nil || !readerEnabled(r) {
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
