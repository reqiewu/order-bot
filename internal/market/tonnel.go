package market

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/reqiewu/order-bot/internal/giftid"
	"github.com/reqiewu/order-bot/internal/marketport"
)

const (
	defaultTonnelBaseURL = "https://gifts3.tonnel.network"
	tonnelMaxPages       = 5 // ~100–150 лотов при page 30
	tonnelListPageSize   = 30
	tonnelSalesPageSize  = 50
	tonnelGiftURLPrefix  = "https://marketplace.tonnel.network/nft/"
)

// TonnelConfig — gifts.tonnel.network (pageGifts + saleHistory).
type TonnelConfig struct {
	BaseURL   string
	InitData  string       // Telegram WebApp initData; List ок без него, RecentSales — нет
	HTTP      *http.Client // если задан (тесты) — обычный net/http; иначе Chrome TLS
	MaxPages  int
	PageSize  int
	ListLimit int
}

// Tonnel — ридер витрины Tonnel (asks через pageGifts, comps через saleHistory).
type Tonnel struct {
	baseURL   string
	doer      tonnelDoer
	maxPages  int
	pageSize  int
	listLimit int
	gate      tonnelGate

	authMu   sync.RWMutex
	initData string
}

func NewTonnel(cfg TonnelConfig) *Tonnel {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = defaultTonnelBaseURL
	}
	maxPages := cfg.MaxPages
	if maxPages <= 0 {
		maxPages = tonnelMaxPages
	}
	pageSize := cfg.PageSize
	if pageSize <= 0 {
		pageSize = tonnelListPageSize
	}
	listLimit := cfg.ListLimit
	if listLimit <= 0 {
		listLimit = marketport.DefaultListLimit
	}
	return &Tonnel{
		baseURL:   base,
		initData:  strings.TrimSpace(cfg.InitData),
		doer:      newTonnelDoer(cfg.HTTP),
		maxPages:  maxPages,
		pageSize:  pageSize,
		listLimit: listLimit,
	}
}

var _ marketport.MarketReader = (*Tonnel)(nil)

// SetInitData обновляет Telegram initData без рестарта (Mini App / comps).
func (t *Tonnel) SetInitData(initData string) {
	if t == nil {
		return
	}
	t.authMu.Lock()
	t.initData = strings.TrimSpace(initData)
	t.authMu.Unlock()
}

// InitData — текущий authData (для Live-статуса Mini App).
func (t *Tonnel) InitData() string {
	if t == nil {
		return ""
	}
	t.authMu.RLock()
	defer t.authMu.RUnlock()
	return t.initData
}

type tonnelGift struct {
	GiftID           json.RawMessage `json:"gift_id"`
	GiftNum          int             `json:"gift_num"`
	Name             string          `json:"name"`
	Model            string          `json:"model"`
	Backdrop         string          `json:"backdrop"`
	Symbol           string          `json:"symbol"`
	Price            float64         `json:"price"`
	Asset            string          `json:"asset"`
	Status           string          `json:"status"`
	AuctionID        json.RawMessage `json:"auction_id"`
	DutchAuctionData json.RawMessage `json:"dutchAuctionData"`
	Buyer            json.RawMessage `json:"buyer"`
	Refunded         bool            `json:"refunded"`
}

type tonnelSaleEl struct {
	GiftName  string          `json:"gift_name"`
	GiftNum   int             `json:"gift_num"`
	Model     string          `json:"model"`
	Backdrop  string          `json:"backdrop"`
	Price     float64         `json:"price"`
	Asset     string          `json:"asset"`
	Type      string          `json:"type"`
	Timestamp json.RawMessage `json:"timestamp"`
}

type tonnelErrEnvelope struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (t *Tonnel) HasInitData() bool {
	return strings.TrimSpace(t.InitData()) != ""
}

func (t *Tonnel) Enabled() bool { return t != nil && t.HasInitData() }

func (t *Tonnel) CheckAuth(ctx context.Context) error {
	_, err := t.pageGifts(ctx, 1, 1, map[string]any{
		"price": map[string]any{"$exists": true},
		"buyer": map[string]any{"$exists": false},
	})
	return err
}

func (t *Tonnel) List(ctx context.Context, watch marketport.WatchItem) ([]marketport.Listing, error) {
	if strings.TrimSpace(watch.Collection) == "" {
		return nil, fmt.Errorf("market/tonnel: empty collection")
	}
	filter := tonnelMarketFilter(watch)
	wantColl := giftid.Fold(watch.Collection)
	wantModel := giftid.Fold(stripTonnelRarity(watch.Model))
	wantBG := giftid.Fold(stripTonnelRarity(watch.Backdrop))
	var out []marketport.Listing
	for page := 1; page <= t.maxPages && len(out) < t.listLimit; page++ {
		items, err := t.pageGifts(ctx, page, t.pageSize, filter)
		if err != nil {
			return nil, err
		}
		for _, g := range items {
			lot, ok := normalizeTonnelGift(g, watch.Collection)
			if !ok {
				continue
			}
			if giftid.Fold(lot.Collection) != wantColl {
				continue
			}
			if wantModel != "" && giftid.Fold(lot.Model) != wantModel {
				continue
			}
			if wantBG != "" && giftid.Fold(lot.Backdrop) != wantBG {
				continue
			}
			out = append(out, lot)
			if len(out) >= t.listLimit {
				break
			}
		}
		if len(items) < t.pageSize {
			break
		}
	}
	return marketport.TakeCheapest(out, t.listLimit), nil
}

func (t *Tonnel) RecentSales(ctx context.Context, like marketport.Listing, limit int) ([]marketport.Sale, error) {
	if !t.HasInitData() {
		return nil, ErrEmptyToken
	}
	if limit <= 0 {
		limit = marketport.DefaultSaleLimit
	}
	filter := tonnelSalesFilter(like)
	wantModel := giftid.Fold(stripTonnelRarity(like.Model))
	wantBG := giftid.Fold(stripTonnelRarity(like.Backdrop))
	var out []marketport.Sale
	for page := 1; page <= t.maxPages && len(out) < limit; page++ {
		items, err := t.saleHistory(ctx, page, tonnelSalesPageSize, filter)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		for _, el := range items {
			sale, ok := normalizeTonnelSale(el, like.Collection)
			if !ok {
				continue
			}
			// Пустые model/backdrop после SALE — collection-wide comps, не дропаем.
			if wantModel != "" && sale.Model != "" && giftid.Fold(sale.Model) != wantModel {
				continue
			}
			if wantBG != "" && sale.Backdrop != "" && giftid.Fold(sale.Backdrop) != wantBG {
				continue
			}
			out = append(out, sale)
			if len(out) >= limit {
				break
			}
		}
		if len(items) < tonnelSalesPageSize {
			break
		}
	}
	return out, nil
}

func (t *Tonnel) pageGifts(ctx context.Context, page, limit int, filter map[string]any) ([]tonnelGift, error) {
	filterRaw, err := json.Marshal(filter)
	if err != nil {
		return nil, err
	}
	sortRaw, err := json.Marshal(struct {
		Price  int `json:"price"`
		GiftID int `json:"gift_id"`
	}{Price: 1, GiftID: -1})
	if err != nil {
		return nil, err
	}
	body, err := t.post(ctx, "/api/pageGifts", map[string]any{
		"page":        page,
		"limit":       limit,
		"sort":        string(sortRaw),
		"filter":      string(filterRaw),
		"ref":         0,
		"price_range": nil,
		"user_auth":   t.InitData(),
	})
	if err != nil {
		return nil, err
	}
	var items []tonnelGift
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("market/tonnel: decode pageGifts: %w", err)
	}
	return items, nil
}

func (t *Tonnel) saleHistory(ctx context.Context, page, limit int, filter map[string]any) ([]tonnelSaleEl, error) {
	body, err := t.post(ctx, "/api/saleHistory", map[string]any{
		"authData": t.InitData(),
		"page":     page,
		"limit":    limit,
		"type":     "SALE",
		"filter":   filter,
		"sort": struct {
			Timestamp int `json:"timestamp"`
			GiftID    int `json:"gift_id"`
		}{Timestamp: -1, GiftID: -1},
	})
	if err != nil {
		return nil, err
	}
	var items []tonnelSaleEl
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("market/tonnel: decode saleHistory: %w", err)
	}
	return items, nil
}

func (t *Tonnel) post(ctx context.Context, path string, payload any) (json.RawMessage, error) {
	if err := t.gate.wait(ctx); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	status, body, err := t.doer.Do(ctx, http.MethodPost, t.baseURL+path, raw)
	if err != nil {
		return nil, fmt.Errorf("market/tonnel: http: %w", err)
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, &UnauthorizedError{Cause: fmt.Errorf("status %d: %s", status, truncate(body, 200))}
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("market/tonnel: status %d: %s", status, truncate(body, 200))
	}
	if err := tonnelAPIError(body); err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}

func tonnelAPIError(body []byte) error {
	trim := bytes.TrimSpace(body)
	if len(trim) == 0 || trim[0] != '{' {
		return nil
	}
	var env tonnelErrEnvelope
	if err := json.Unmarshal(trim, &env); err != nil {
		return nil
	}
	if !strings.EqualFold(env.Status, "error") {
		return nil
	}
	msg := strings.TrimSpace(env.Message)
	if msg == "" {
		msg = truncate(trim, 200)
	}
	if strings.Contains(strings.ToLower(msg), "auth") {
		return &UnauthorizedError{Cause: fmt.Errorf("market/tonnel: %s", msg)}
	}
	return fmt.Errorf("market/tonnel: %s", msg)
}

func tonnelMarketFilter(watch marketport.WatchItem) map[string]any {
	// MarketPage defaultFilter + GiftGrid: gift_name / modelName / backdrop / asset=TON.
	f := map[string]any{
		"price":     map[string]any{"$exists": true},
		"buyer":     map[string]any{"$exists": false},
		"gift_name": strings.TrimSpace(watch.Collection),
		"asset":     "TON",
	}
	if m := stripTonnelRarity(watch.Model); m != "" {
		f["modelName"] = m
	}
	if bg := tonnelTraitFilter(watch.Backdrop); bg != nil {
		f["backdrop"] = bg
	}
	return f
}

func tonnelSalesFilter(like marketport.Listing) map[string]any {
	f := map[string]any{}
	if coll := strings.TrimSpace(like.Collection); coll != "" {
		f["gift_name"] = coll
	}
	if m := tonnelTraitFilter(like.Model); m != nil {
		f["model"] = m
	}
	if bg := tonnelTraitFilter(like.Backdrop); bg != nil {
		f["backdrop"] = bg
	}
	return f
}

func tonnelTraitFilter(name string) any {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if strings.Contains(name, "%") {
		return name
	}
	name = stripTonnelRarity(name)
	if name == "" {
		return nil
	}
	return map[string]string{"$regex": "^" + regexp.QuoteMeta(name) + ` \(`}
}

func normalizeTonnelGift(g tonnelGift, fallback string) (marketport.Listing, bool) {
	if g.Refunded || jsonPresent(g.Buyer) || jsonPresent(g.AuctionID) || jsonPresent(g.DutchAuctionData) {
		return marketport.Listing{}, false
	}
	if strings.EqualFold(g.Status, "auction") {
		return marketport.Listing{}, false
	}
	if g.Asset != "" && !strings.EqualFold(g.Asset, "TON") {
		return marketport.Listing{}, false
	}
	if g.Price <= 0 {
		return marketport.Listing{}, false
	}
	id := stringifyJSONID(g.GiftID)
	if id == "" {
		return marketport.Listing{}, false
	}
	coll := strings.TrimSpace(g.Name)
	if coll == "" {
		coll = fallback
	}
	var num *int
	if g.GiftNum > 0 {
		n := g.GiftNum
		num = &n
	}
	return marketport.Listing{
		ID:         id,
		Collection: coll,
		Model:      stripTonnelRarity(g.Model),
		Price:      g.Price,
		Number:     num,
		Backdrop:   stripTonnelRarity(g.Backdrop),
		Symbol:     stripTonnelRarity(g.Symbol),
		URL:        tonnelGiftURLPrefix + id,
		Market:     marketport.MarketTonnel,
	}, true
}

func normalizeTonnelSale(el tonnelSaleEl, fallback string) (marketport.Sale, bool) {
	if el.Type != "" && !strings.EqualFold(el.Type, "SALE") && !strings.EqualFold(el.Type, "INTERNAL_SALE") {
		return marketport.Sale{}, false
	}
	if el.Asset != "" && !strings.EqualFold(el.Asset, "TON") {
		return marketport.Sale{}, false
	}
	if el.Price <= 0 {
		return marketport.Sale{}, false
	}
	coll := strings.TrimSpace(el.GiftName)
	if coll == "" {
		coll = fallback
	}
	var num *int
	if el.GiftNum > 0 {
		n := el.GiftNum
		num = &n
	}
	return marketport.Sale{
		Price:      el.Price,
		Number:     num,
		At:         parseTonnelTime(el.Timestamp),
		Collection: coll,
		Model:      stripTonnelRarity(el.Model),
		Backdrop:   stripTonnelRarity(el.Backdrop),
	}, true
}

func stripTonnelRarity(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, " ("); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func jsonPresent(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s != "" && s != "null" && s != "false" && s != `""`
}

func stringifyJSONID(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	if len(s) >= 2 && s[0] == '"' {
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			return strings.TrimSpace(str)
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return strconv.FormatInt(n, 10)
	}
	f, err := strconv.ParseFloat(s, 64)
	if err == nil && f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strings.Trim(s, `"`)
}

func parseTonnelTime(raw json.RawMessage) time.Time {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return time.Time{}
	}
	if len(s) >= 2 && s[0] == '"' {
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			if t, err := time.Parse(time.RFC3339, str); err == nil {
				return t
			}
			if t, err := time.Parse(time.RFC3339Nano, str); err == nil {
				return t
			}
			if n, err := strconv.ParseInt(str, 10, 64); err == nil {
				return unixFlexible(n)
			}
		}
		return time.Time{}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return time.Time{}
		}
		n = int64(f)
	}
	return unixFlexible(n)
}

func unixFlexible(n int64) time.Time {
	if n <= 0 {
		return time.Time{}
	}
	if n > 1e12 {
		return time.UnixMilli(n).UTC()
	}
	return time.Unix(n, 0).UTC()
}

// ProbeTonnel — pageGifts отвечает (initData не обязателен для витрины).
func ProbeTonnel(ctx context.Context, initData string) error {
	return NewTonnel(TonnelConfig{InitData: strings.TrimSpace(initData)}).CheckAuth(ctx)
}

// ProbeTonnelInitData — saleHistory принимает authData (для comps / Mini App save).
func ProbeTonnelInitData(ctx context.Context, initData string) error {
	initData = NormalizePortalsTMA(initData)
	if initData == "" {
		return ErrEmptyToken
	}
	t := NewTonnel(TonnelConfig{InitData: initData})
	_, err := t.RecentSales(ctx, marketport.Listing{Collection: "Fine Pen"}, 1)
	return err
}
