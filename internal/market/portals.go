package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/reqiewu/order-bot/internal/giftid"
	"github.com/reqiewu/order-bot/internal/marketport"
)

const (
	defaultPortalsBaseURL = "https://portal-market.com"
	defaultPortalsPages   = 20 // ~ListLimit / pageSize
	portalsPageSize       = 20
)

// PortalsConfig — HTTP-клиент Portals (portal-market.com).
type PortalsConfig struct {
	BaseURL   string
	Auth      TokenProvider // TMA initData; для RecentSales обязателен
	HTTP      *http.Client
	MaxPages  int
	PageSize  int
	ListLimit int
}

// Portals — адаптер MarketReader для Portals.
type Portals struct {
	baseURL   string
	auth      TokenProvider
	http      *http.Client
	maxPages  int
	pageSize  int
	listLimit int
	gate      portalsGate
	listMu    sync.Mutex
	listCache map[string]portalsListCacheEntry

	colMu      sync.Mutex
	colByShort map[string]string // short_name / fold → uuid
	colFetched time.Time
}

type portalsListCacheEntry struct {
	at       time.Time
	listings []marketport.Listing
}

const (
	portalsListCacheTTL = time.Second
	portalsColCacheTTL  = 30 * time.Minute
)

type portalsCollectionsResponse struct {
	Collections []portalsCollectionMeta `json:"collections"`
}

type portalsCollectionMeta struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
}

func portalsListCacheKey(watch marketport.WatchItem) string {
	names := giftid.PortalsNames(giftid.Identity{
		Collection: watch.Collection,
		Model:      watch.Model,
		Backdrop:   watch.Backdrop,
	})
	return names.Collection + "\x00" + names.Model + "\x00" + names.Backdrop
}

// NewPortals создаёт клиент Portals.
func NewPortals(cfg PortalsConfig) *Portals {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = defaultPortalsBaseURL
	}
	httpClient := cfg.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	maxPages := cfg.MaxPages
	if maxPages <= 0 {
		maxPages = defaultPortalsPages
	}
	pageSize := cfg.PageSize
	if pageSize <= 0 {
		pageSize = portalsPageSize
	}
	listLimit := cfg.ListLimit
	if listLimit <= 0 {
		listLimit = marketport.DefaultListLimit
	}
	return &Portals{
		baseURL:   base,
		auth:      cfg.Auth,
		http:      httpClient,
		maxPages:  maxPages,
		pageSize:  pageSize,
		listLimit: listLimit,
	}
}

func (p *Portals) Enabled() bool { return p != nil && tokenPresent(p.auth) }

var _ marketport.MarketReader = (*Portals)(nil)

type portalsSearchResponse struct {
	Results []portalsNFT `json:"results"`
}

type portalsActionsResponse struct {
	Actions []portalsAction `json:"actions"`
}

type portalsAction struct {
	Type      string          `json:"type"`
	Amount    json.RawMessage `json:"amount"`
	CreatedAt string          `json:"created_at"`
	NFT       portalsNFT      `json:"nft"`
}

type portalsNFT struct {
	ID                       string             `json:"id"`
	TGID                     string             `json:"tg_id"`
	Name                     string             `json:"name"`
	ExternalCollectionNumber *int               `json:"external_collection_number"`
	Price                    json.RawMessage    `json:"price"`
	Attributes               []portalsAttribute `json:"attributes"`
	PhotoURL                 string             `json:"photo_url"`
}

type portalsAttribute struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// List — GET /api/nfts/search (listings; auth опционален по research).
// Короткий TTL-кэш снижает дубли между Radar-запросами на одном tick.
func (p *Portals) List(ctx context.Context, watch marketport.WatchItem) ([]marketport.Listing, error) {
	if watch.Collection == "" {
		return nil, fmt.Errorf("market/portals: empty collection")
	}
	key := portalsListCacheKey(watch)
	p.listMu.Lock()
	if p.listCache != nil {
		if e, ok := p.listCache[key]; ok && time.Since(e.at) < portalsListCacheTTL {
			out := append([]marketport.Listing(nil), e.listings...)
			p.listMu.Unlock()
			return out, nil
		}
	}
	p.listMu.Unlock()

	out, err := p.listUncached(ctx, watch)
	if err != nil {
		return nil, err
	}
	p.listMu.Lock()
	if p.listCache == nil {
		p.listCache = map[string]portalsListCacheEntry{}
	}
	p.listCache[key] = portalsListCacheEntry{at: time.Now(), listings: append([]marketport.Listing(nil), out...)}
	p.listMu.Unlock()
	return out, nil
}

func (p *Portals) listUncached(ctx context.Context, watch marketport.WatchItem) ([]marketport.Listing, error) {
	names := giftid.PortalsNames(giftid.Identity{
		Collection: watch.Collection,
		Model:      watch.Model,
		Backdrop:   watch.Backdrop,
	})
	colID, err := p.resolveCollectionID(ctx, watch.Collection)
	if err != nil {
		return nil, err
	}
	var out []marketport.Listing
	for page := 0; page < p.maxPages && len(out) < p.listLimit; page++ {
		offset := page * p.pageSize
		q := url.Values{}
		q.Set("offset", strconv.Itoa(offset))
		q.Set("limit", strconv.Itoa(p.pageSize))
		q.Set("status", "listed")
		q.Set("sort_by", "price asc")
		q.Set("exclude_bundled", "true")
		// filter_by_collections(short_name) на live API игнорируется → чужие лоты.
		// Рабочий фильтр: collection_ids=<uuid>.
		q.Set("collection_ids", colID)
		if names.Model != "" {
			q.Set("filter_by_models", names.Model)
		}
		if names.Backdrop != "" {
			q.Set("filter_by_backdrops", names.Backdrop)
		}
		raw, err := p.get(ctx, "/api/nfts/search", q, false)
		if err != nil {
			return nil, err
		}
		var parsed portalsSearchResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("market/portals: decode search: %w", err)
		}
		if len(parsed.Results) == 0 {
			break
		}
		for _, n := range parsed.Results {
			listing, ok := normalizePortalsNFT(n, watch.Collection)
			if !ok {
				continue
			}
			// API иногда отдаёт чужие коллекции при кривом фильтре — отсекаем.
			if !giftid.SameCollection(listing.Collection, watch.Collection) {
				continue
			}
			out = append(out, listing)
			if len(out) >= p.listLimit {
				break
			}
		}
		if len(parsed.Results) < p.pageSize {
			break
		}
	}
	return marketport.TakeCheapest(out, p.listLimit), nil
}

func (p *Portals) resolveCollectionID(ctx context.Context, collection string) (string, error) {
	if err := p.ensureCollections(ctx); err != nil {
		return "", err
	}
	short := giftid.PortalsCollection(collection)
	fold := giftid.Fold(collection)
	p.colMu.Lock()
	defer p.colMu.Unlock()
	if id := p.colByShort[short]; id != "" {
		return id, nil
	}
	if id := p.colByShort[fold]; id != "" {
		return id, nil
	}
	return "", fmt.Errorf("market/portals: unknown collection %q (short %q)", collection, short)
}

func (p *Portals) ensureCollections(ctx context.Context) error {
	p.colMu.Lock()
	if p.colByShort != nil && time.Since(p.colFetched) < portalsColCacheTTL {
		p.colMu.Unlock()
		return nil
	}
	p.colMu.Unlock()

	q := url.Values{}
	q.Set("limit", "500")
	raw, err := p.get(ctx, "/api/collections/", q, false)
	if err != nil {
		return fmt.Errorf("market/portals: collections: %w", err)
	}
	var parsed portalsCollectionsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("market/portals: decode collections: %w", err)
	}
	byShort := make(map[string]string, len(parsed.Collections)*2)
	for _, c := range parsed.Collections {
		if c.ID == "" {
			continue
		}
		if c.ShortName != "" {
			byShort[strings.ToLower(c.ShortName)] = c.ID
			byShort[giftid.Fold(c.ShortName)] = c.ID
		}
		if c.Name != "" {
			byShort[giftid.Fold(c.Name)] = c.ID
			byShort[giftid.PortalsCollection(c.Name)] = c.ID
		}
	}
	p.colMu.Lock()
	p.colByShort = byShort
	p.colFetched = time.Now()
	p.colMu.Unlock()
	return nil
}

// CheckAuth проверяет TMA: лёгкий GET /api/market/actions/ (без auth → 401).
func (p *Portals) CheckAuth(ctx context.Context) error {
	if p == nil {
		return fmt.Errorf("market/portals: nil client")
	}
	if p.auth == nil {
		return fmt.Errorf("market/portals: auth provider is required")
	}
	q := url.Values{}
	q.Set("offset", "0")
	q.Set("limit", "1")
	q.Set("action_types", "sell")
	_, err := p.get(ctx, "/api/market/actions/", q, true)
	if err != nil {
		return err
	}
	return nil
}

// RecentSales — GET /api/market/actions/ (требует TMA).
// Live UI/DevTools: action_types=sell + collection_ids + filter_by_models/backdrops;
// в JSON сделки приходят как type=purchase.
func (p *Portals) RecentSales(ctx context.Context, like marketport.Listing, limit int) ([]marketport.Sale, error) {
	if p.auth == nil {
		return nil, fmt.Errorf("market/portals: auth provider is required for sales")
	}
	if limit <= 0 {
		limit = marketport.DefaultSaleLimit
	}
	names := giftid.PortalsNames(giftid.Identity{
		Collection: like.Collection,
		Model:      like.Model,
		Backdrop:   like.Backdrop,
	})
	colID := ""
	if like.Collection != "" {
		var err error
		colID, err = p.resolveCollectionID(ctx, like.Collection)
		if err != nil {
			return nil, err
		}
	}
	var out []marketport.Sale
	for page := 0; page < p.maxPages && len(out) < limit; page++ {
		offset := page * p.pageSize
		q := url.Values{}
		q.Set("offset", strconv.Itoa(offset))
		q.Set("limit", strconv.Itoa(p.pageSize))
		q.Set("action_types", "sell")
		if colID != "" {
			q.Set("collection_ids", colID)
		}
		if names.Model != "" {
			q.Set("filter_by_models", names.Model)
		}
		if names.Backdrop != "" {
			q.Set("filter_by_backdrops", names.Backdrop)
		}
		raw, err := p.get(ctx, "/api/market/actions/", q, true)
		if err != nil {
			return nil, err
		}
		var parsed portalsActionsResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("market/portals: decode actions: %w", err)
		}
		if len(parsed.Actions) == 0 {
			break
		}
		for _, a := range parsed.Actions {
			if !portalsIsSaleAction(a.Type) {
				continue
			}
			sale, ok := normalizePortalsAction(a)
			if !ok {
				continue
			}
			if !portalsSaleMatches(like, sale) {
				continue
			}
			out = append(out, sale)
			if len(out) >= limit {
				break
			}
		}
		if len(parsed.Actions) < p.pageSize {
			break
		}
	}
	// API отдаёт новые сверху; в алерте — старые → новые.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// portalsIsSaleAction — live: purchase (+ legacy buy/sell в фикстурах).
func portalsIsSaleAction(typ string) bool {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "", "purchase", "buy", "sell":
		return true
	default:
		return false
	}
}

func (p *Portals) get(ctx context.Context, path string, q url.Values, requireAuth bool) ([]byte, error) {
	u := p.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}

	var lastErr error
	for attempt := 0; attempt <= portalsMaxRetries; attempt++ {
		if err := p.gate.wait(ctx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("market/portals: new request: %w", err)
		}
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Origin", "https://portal-market.com")
		req.Header.Set("Referer", "https://portal-market.com/")
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; order-bot/1.0)")
		if requireAuth {
			if p.auth == nil {
				return nil, fmt.Errorf("market/portals: auth provider is required")
			}
			tok, err := p.auth.Token(ctx)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", portalsAuthHeader(tok))
		} else if p.auth != nil {
			// Public endpoints work without TMA; attach token only if present.
			if tok, err := p.auth.Token(ctx); err == nil && tok != "" {
				req.Header.Set("Authorization", portalsAuthHeader(tok))
			}
		}

		res, err := p.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("market/portals: http: %w", err)
		}
		raw, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("market/portals: read body: %w", err)
		}
		if res.StatusCode == http.StatusUnauthorized {
			return nil, &UnauthorizedError{Cause: fmt.Errorf("status %d: %s", res.StatusCode, truncate(raw, 200))}
		}
		if res.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("market/portals: unexpected status %d: %s", res.StatusCode, truncate(raw, 200))
			if attempt == portalsMaxRetries {
				break
			}
			delay := retryAfterDelay(res.Header.Get("Retry-After"), attempt)
			t := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return nil, ctx.Err()
			case <-t.C:
			}
			continue
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, fmt.Errorf("market/portals: unexpected status %d: %s", res.StatusCode, truncate(raw, 200))
		}
		return raw, nil
	}
	return nil, lastErr
}

func portalsAuthHeader(tok string) string {
	tok = strings.TrimSpace(tok)
	tok = strings.Trim(tok, `"'`)
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(tok), "tma ") {
		return "tma " + strings.TrimSpace(tok[4:])
	}
	return "tma " + tok
}

func normalizePortalsNFT(n portalsNFT, fallbackCollection string) (marketport.Listing, bool) {
	id := firstNonEmpty(n.ID, n.TGID)
	if id == "" {
		return marketport.Listing{}, false
	}
	price := parsePortalsPrice(n.Price)
	if price <= 0 {
		return marketport.Listing{}, false
	}
	model, backdrop, symbol := portalsAttrs(n.Attributes)
	collection := firstNonEmpty(n.Name, fallbackCollection)
	return marketport.Listing{
		ID:         id,
		Collection: collection,
		Model:      model,
		Price:      price,
		Number:     n.ExternalCollectionNumber,
		Backdrop:   backdrop,
		Symbol:     symbol,
		URL:        portalsMarketURL(n),
	}, true
}

// portalsMarketURL — ссылка «Share» из Portals Mini App: startapp=gift_<nft uuid>.
// startapp=<tg_id> (FinePen-15525) только открывает пустой маркет; t.me/nft/ — в FormatAlert.
func portalsMarketURL(n portalsNFT) string {
	if id := strings.TrimSpace(n.ID); id != "" {
		return "https://t.me/portals_market_bot/market?startapp=gift_" + id
	}
	tg := strings.TrimSpace(n.TGID)
	if tg == "" && n.ExternalCollectionNumber != nil {
		tg = giftid.TelegramGiftID(firstNonEmpty(n.Name, ""), *n.ExternalCollectionNumber)
	}
	if tg != "" {
		return "https://t.me/nft/" + tg
	}
	return "https://t.me/portals_market_bot/market"
}

func normalizePortalsAction(a portalsAction) (marketport.Sale, bool) {
	price := parsePortalsPrice(a.Amount)
	if price <= 0 {
		return marketport.Sale{}, false
	}
	model, backdrop, _ := portalsAttrs(a.NFT.Attributes)
	collection := a.NFT.Name
	at, _ := time.Parse(time.RFC3339Nano, a.CreatedAt)
	if at.IsZero() {
		at, _ = time.Parse(time.RFC3339, a.CreatedAt)
	}
	return marketport.Sale{
		Price:      price,
		Number:     a.NFT.ExternalCollectionNumber,
		At:         at,
		Collection: collection,
		Model:      model,
		Backdrop:   backdrop,
	}, true
}

func portalsSaleMatches(like marketport.Listing, sale marketport.Sale) bool {
	if like.Collection != "" && !giftid.SameCollection(like.Collection, sale.Collection) {
		return false
	}
	if like.Model != "" && sale.Model != like.Model {
		return false
	}
	if like.Backdrop != "" && sale.Backdrop != like.Backdrop {
		return false
	}
	return true
}

func portalsAttrs(attrs []portalsAttribute) (model, backdrop, symbol string) {
	for _, a := range attrs {
		switch strings.ToLower(a.Type) {
		case "model":
			model = a.Value
		case "backdrop":
			backdrop = a.Value
		case "symbol":
			symbol = a.Value
		}
	}
	return model, backdrop, symbol
}

func parsePortalsPrice(raw json.RawMessage) float64 {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return v
	}
	return 0
}
