package engine

import (
	"log/slog"
	"sync"
	"time"

	"github.com/reqiewu/order-bot/internal/catalog"
	"github.com/reqiewu/order-bot/internal/giftid"
	"github.com/reqiewu/order-bot/internal/money"
	"github.com/reqiewu/order-bot/internal/spread"
)

// MarketEvent — пакет обновления от ingress.
type MarketEvent struct {
	Market    string
	WatchID   string
	Listings  []catalog.Lot // полный replace по watch+market
	FetchedAt time.Time
}

// Signal — полный paper-алерт.
type Signal struct {
	BuyMarket   string
	SellMarket  string
	Lot         catalog.Lot
	BestAsk     money.NanoTON
	Undercut    money.NanoTON
	SalesMedian money.NanoTON
	NetAsk      money.NanoTON
	NetSales    money.NanoTON
	CollFloor   money.NanoTON
	At          time.Time
}

// SalesFunc тянет sales для кандидата (on-demand).
type SalesFunc func(sellMarket string, like catalog.Lot) ([]money.NanoTON, error)

// Alerter доставляет сигнал оператору.
type Alerter interface {
	PaperSignal(s Signal) error
}

// Deduper — A: новый ок / плохой→ок; уже ок — скип.
type Deduper interface {
	ShouldAlert(market, listingID string, buyPrice money.NanoTON, triggered bool) bool
	Remember(market, listingID string, buyPrice money.NanoTON, triggered bool)
	ForgetMissing(market string, liveIDs map[string]struct{})
}

// MemoryDeduper — in-memory; store может обернуть persist.
type MemoryDeduper struct {
	mu   sync.Mutex
	seen map[string]dedupEntry
}

type dedupEntry struct {
	price     money.NanoTON
	triggered bool
}

func NewMemoryDeduper() *MemoryDeduper {
	return &MemoryDeduper{seen: make(map[string]dedupEntry)}
}

func dedupKey(market, id string) string { return market + "\x00" + id }

func (d *MemoryDeduper) ShouldAlert(market, listingID string, buyPrice money.NanoTON, triggered bool) bool {
	if !triggered {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	e, ok := d.seen[dedupKey(market, listingID)]
	if !ok {
		return true // новый + ок
	}
	if !e.triggered {
		return true // был плохой → стал ок
	}
	return false // уже алертили как ок
}

func (d *MemoryDeduper) Remember(market, listingID string, buyPrice money.NanoTON, triggered bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen[dedupKey(market, listingID)] = dedupEntry{price: buyPrice, triggered: triggered}
}

func (d *MemoryDeduper) ForgetMissing(market string, liveIDs map[string]struct{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	prefix := market + "\x00"
	for k := range d.seen {
		if len(k) < len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		id := k[len(prefix):]
		if _, ok := liveIDs[id]; !ok {
			delete(d.seen, k)
		}
	}
}

// Engine держит книги и считает кросс Portals↔MRKT.
type Engine struct {
	log    *slog.Logger
	fees   spread.Fees
	sales  SalesFunc
	alert  Alerter
	dedup  Deduper

	mu   sync.Mutex
	book map[string]map[string]catalog.Lot // market → listingID → lot
}

func New(log *slog.Logger, fees spread.Fees, sales SalesFunc, alert Alerter, dedup Deduper) *Engine {
	if log == nil {
		log = slog.Default()
	}
	if dedup == nil {
		dedup = NewMemoryDeduper()
	}
	return &Engine{
		log:   log,
		fees:  fees,
		sales: sales,
		alert: alert,
		dedup: dedup,
		book:  map[string]map[string]catalog.Lot{},
	}
}

func (e *Engine) Handle(ev MarketEvent) {
	e.mu.Lock()
	if e.book[ev.Market] == nil {
		e.book[ev.Market] = map[string]catalog.Lot{}
	}
	// replace: удалить лоты этого watch? для простоты v1 — merge/replace by listing id from event,
	// и пометить live set для forget. Полный replace маркета по watch требует метки watch на лоте.
	live := map[string]struct{}{}
	for _, lot := range ev.Listings {
		lot.Market = ev.Market
		e.book[ev.Market][lot.ListingID] = lot
		live[lot.ListingID] = struct{}{}
	}
	// снимок для расчёта вне лока
	buyMarket := ev.Market
	candidates := append([]catalog.Lot(nil), ev.Listings...)
	e.mu.Unlock()

	e.dedup.ForgetMissing(buyMarket, live)

	sellMarket := otherMarket(buyMarket)
	if sellMarket == "" {
		return
	}

	for _, lot := range candidates {
		e.evalLot(lot, sellMarket)
	}
}

func otherMarket(m string) string {
	switch m {
	case "mrkt":
		return "portals"
	case "portals":
		return "mrkt"
	default:
		return ""
	}
}

func (e *Engine) evalLot(lot catalog.Lot, sellMarket string) {
	e.mu.Lock()
	bestAsk, collFloor, ok := e.bestAskLocked(sellMarket, lot.ModelBG)
	e.mu.Unlock()
	if !ok {
		e.dedup.Remember(lot.Market, lot.ListingID, lot.Price, false)
		return
	}

	askEval := spread.EvalFromAsk(lot.Price, bestAsk, e.fees)
	if !askEval.Triggered {
		e.dedup.Remember(lot.Market, lot.ListingID, lot.Price, false)
		return
	}

	if e.sales == nil {
		return
	}
	raw, err := e.sales(sellMarket, lot)
	if err != nil {
		e.log.Debug("sales fetch failed", "err", err, "lot", lot.ListingID)
		return
	}
	salesEval := spread.EvalFromSales(lot.Price, raw, bestAsk, e.fees)
	triggered := spread.FullTrigger(askEval, salesEval)
	if !triggered {
		e.dedup.Remember(lot.Market, lot.ListingID, lot.Price, false)
		return
	}
	if !e.dedup.ShouldAlert(lot.Market, lot.ListingID, lot.Price, true) {
		return
	}
	e.dedup.Remember(lot.Market, lot.ListingID, lot.Price, true)

	if e.alert == nil {
		return
	}
	sig := Signal{
		BuyMarket:   lot.Market,
		SellMarket:  sellMarket,
		Lot:         lot,
		BestAsk:     askEval.BestAsk,
		Undercut:    askEval.Undercut,
		SalesMedian: salesEval.Median,
		NetAsk:      askEval.Net,
		NetSales:    salesEval.Net,
		CollFloor:   collFloor,
		At:          time.Now().UTC(),
	}
	if err := e.alert.PaperSignal(sig); err != nil {
		e.log.Error("alert failed", "err", err)
	}
}

func (e *Engine) bestAskLocked(market string, key catalog.ModelBG) (best money.NanoTON, collFloor money.NanoTON, ok bool) {
	lots := e.book[market]
	want := key.Key()
	coll := giftid.Fold(key.Collection)
	firstColl := true
	for _, lot := range lots {
		if giftid.Fold(lot.ModelBG.Collection) == coll {
			if firstColl || lot.Price < collFloor {
				collFloor = lot.Price
				firstColl = false
			}
		}
		if lot.ModelBG.Key() != want {
			continue
		}
		if !ok || lot.Price < best {
			best = lot.Price
			ok = true
		}
	}
	return best, collFloor, ok
}
