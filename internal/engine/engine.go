package engine

import (
	"sync"
	"time"

	"github.com/reqiewu/order-bot/internal/applog"
	"github.com/reqiewu/order-bot/internal/catalog"
	"github.com/reqiewu/order-bot/internal/giftid"
	"github.com/reqiewu/order-bot/internal/marketport"
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
	SellURL     string // URL лучшего ask на sell-маркете
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

// Engine держит книги и считает кросс Portals ↔ MRKT ↔ Getgems ↔ Tonnel.
type Engine struct {
	log   *applog.Logger
	fees  spread.Fees
	sales SalesFunc
	alert Alerter
	dedup Deduper

	mu          sync.Mutex
	book        map[string]map[string]catalog.Lot   // market → listingID → lot
	watchLive   map[string]map[string]struct{}      // market\0watchID → listingIDs
	askByKey    map[string]map[string]catalog.Lot   // market → ModelBG.Key → cheapest lot
	floorByColl map[string]map[string]money.NanoTON // market → fold(collection) → min ask
}

func New(log *applog.Logger, fees spread.Fees, sales SalesFunc, alert Alerter, dedup Deduper) *Engine {
	if log == nil {
		log = applog.Nop()
	}
	if dedup == nil {
		dedup = NewMemoryDeduper()
	}
	return &Engine{
		log:         log,
		fees:        fees,
		sales:       sales,
		alert:       alert,
		dedup:       dedup,
		book:        map[string]map[string]catalog.Lot{},
		watchLive:   map[string]map[string]struct{}{},
		askByKey:    map[string]map[string]catalog.Lot{},
		floorByColl: map[string]map[string]money.NanoTON{},
	}
}

// SetFees обновляет пороги (из Mini App runtime).
func (e *Engine) SetFees(f spread.Fees) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fees = f
}

func (e *Engine) getFees() spread.Fees {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.fees
}

func (e *Engine) Handle(ev MarketEvent) {
	e.mu.Lock()
	if e.book[ev.Market] == nil {
		e.book[ev.Market] = map[string]catalog.Lot{}
	}
	watchKey := ev.Market + "\x00" + ev.WatchID
	prevLive := e.watchLive[watchKey]
	live := map[string]struct{}{}
	for _, lot := range ev.Listings {
		lot.Market = ev.Market
		e.book[ev.Market][lot.ListingID] = lot
		live[lot.ListingID] = struct{}{}
	}
	for id := range prevLive {
		if _, ok := live[id]; !ok {
			delete(e.book[ev.Market], id)
		}
	}
	e.watchLive[watchKey] = live
	e.rebuildIndexLocked(ev.Market)
	// снимок для расчёта вне лока
	buyMarket := ev.Market
	candidates := append([]catalog.Lot(nil), ev.Listings...)
	e.mu.Unlock()

	e.dedup.ForgetMissing(buyMarket, live)

	if !isBuyVenue(buyMarket) {
		return
	}
	for _, lot := range candidates {
		e.evalLot(lot)
	}
}

func isBuyVenue(m string) bool {
	switch m {
	case marketport.MarketMRKT, marketport.MarketPortals, marketport.MarketGetgems, marketport.MarketTonnel:
		return true
	default:
		return false
	}
}

func quoteVenues(buy string) []string {
	all := []string{marketport.MarketMRKT, marketport.MarketPortals, marketport.MarketGetgems, marketport.MarketTonnel}
	out := make([]string, 0, len(all)-1)
	for _, m := range all {
		if m != buy {
			out = append(out, m)
		}
	}
	return out
}

func (e *Engine) evalLot(lot catalog.Lot) {
	e.mu.Lock()
	var (
		sellMarket string
		bestAsk    money.NanoTON
		sellURL    string
		collFloor  money.NanoTON
		ok         bool
	)
	for _, m := range quoteVenues(lot.Market) {
		ask, url, floor, found := e.bestAskLocked(m, lot.ModelBG)
		if !found {
			continue
		}
		if !ok || ask > bestAsk {
			bestAsk = ask
			sellURL = url
			collFloor = floor
			sellMarket = m
			ok = true
		}
	}
	e.mu.Unlock()
	if !ok {
		e.dedup.Remember(lot.Market, lot.ListingID, lot.Price, false)
		return
	}

	fees := e.getFees()
	askEval := spread.EvalFromAsk(lot.Price, bestAsk, fees)
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
	salesEval := spread.EvalFromSales(lot.Price, raw, bestAsk, fees)
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
		SellURL:     sellURL,
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

func (e *Engine) rebuildIndexLocked(market string) {
	asks := map[string]catalog.Lot{}
	floors := map[string]money.NanoTON{}
	for _, lot := range e.book[market] {
		k := lot.ModelBG.Key()
		if prev, ok := asks[k]; !ok || lot.Price < prev.Price {
			asks[k] = lot
		}
		coll := giftid.Fold(lot.ModelBG.Collection)
		if prev, ok := floors[coll]; !ok || lot.Price < prev {
			floors[coll] = lot.Price
		}
	}
	e.askByKey[market] = asks
	e.floorByColl[market] = floors
}

func (e *Engine) bestAskLocked(market string, key catalog.ModelBG) (best money.NanoTON, sellURL string, collFloor money.NanoTON, ok bool) {
	if lots := e.askByKey[market]; lots != nil {
		if lot, found := lots[key.Key()]; found {
			best = lot.Price
			sellURL = lot.URL
			ok = true
		}
	}
	if floors := e.floorByColl[market]; floors != nil {
		collFloor = floors[giftid.Fold(key.Collection)]
	}
	return best, sellURL, collFloor, ok
}
