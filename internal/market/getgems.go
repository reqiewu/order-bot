package market

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

var errGetgemsNotFound = errors.New("market/getgems: not found")

const (
	defaultGetgemsBaseURL = "https://api.getgems.io/public-api/v1"
	getgemsMaxPages       = 3 // быстрый poll: не весь стакан
	getgemsPageSize       = 100
	getgemsColCacheTTL    = 30 * time.Minute
	getgemsListCacheTTL   = time.Second
)

// GetgemsConfig — Read API Getgems (нужен API key).
type GetgemsConfig struct {
	BaseURL   string
	APIKey    string
	HTTP      *http.Client
	MaxPages  int
	ListLimit int
}

// Getgems — ридер Telegram Gifts на Getgems (листинги + activity/sold).
type Getgems struct {
	baseURL   string
	http      *http.Client
	maxPages  int
	listLimit int
	gate      getgemsGate

	keyMu  sync.RWMutex
	apiKey string

	colMu      sync.Mutex
	colByFold  map[string]string // fold(name) → collection address
	colFetched time.Time

	listMu    sync.Mutex
	listCache map[string]getgemsListCacheEntry // collection address → raw on-sale
}

func NewGetgems(cfg GetgemsConfig) *Getgems {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = defaultGetgemsBaseURL
	}
	client := cfg.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	maxPages := cfg.MaxPages
	if maxPages <= 0 {
		maxPages = getgemsMaxPages
	}
	listLimit := cfg.ListLimit
	if listLimit <= 0 {
		listLimit = marketport.DefaultListLimit
	}
	return &Getgems{
		baseURL:   base,
		apiKey:    strings.TrimSpace(cfg.APIKey),
		http:      client,
		maxPages:  maxPages,
		listLimit: listLimit,
	}
}

var _ marketport.MarketReader = (*Getgems)(nil)

// SetAPIKey обновляет Read API key без рестарта (Mini App).
func (g *Getgems) SetAPIKey(key string) {
	if g == nil {
		return
	}
	g.keyMu.Lock()
	g.apiKey = strings.TrimSpace(key)
	g.keyMu.Unlock()
}

// APIKey — текущий ключ (для Live-статуса Mini App).
func (g *Getgems) APIKey() string {
	if g == nil {
		return ""
	}
	g.keyMu.RLock()
	defer g.keyMu.RUnlock()
	return g.apiKey
}

func (g *Getgems) HasAPIKey() bool {
	return strings.TrimSpace(g.APIKey()) != ""
}

func (g *Getgems) Enabled() bool { return g != nil && g.HasAPIKey() }

type getgemsListCacheEntry struct {
	at    time.Time
	items []getgemsNFT
}

type getgemsEnvelope struct {
	Success  bool            `json:"success"`
	Response json.RawMessage `json:"response"`
}

type getgemsItemsPage struct {
	Cursor string       `json:"cursor"`
	Items  []getgemsNFT `json:"items"`
}

type getgemsNFT struct {
	Address           string             `json:"address"`
	Index             int                `json:"index"`
	Name              string             `json:"name"`
	CollectionAddress string             `json:"collectionAddress"`
	CollectionName    string             `json:"collectionName"`
	Collection        *getgemsCollection `json:"collection"`
	Attributes        []getgemsAttr      `json:"attributes"`
	Sale              *getgemsSale       `json:"sale"`
}

type getgemsCollection struct {
	Address string `json:"address"`
	Name    string `json:"name"`
}

type getgemsAttr struct {
	TraitType string `json:"traitType"`
	Value     string `json:"value"`
}

type getgemsSale struct {
	Type      string `json:"type"`
	FullPrice string `json:"fullPrice"`
	Currency  string `json:"currency"`
}

type getgemsHistoryPage struct {
	Cursor string             `json:"cursor"`
	Items  []getgemsHistoryEl `json:"items"`
}

type getgemsHistoryEl struct {
	Address           string          `json:"address"`
	Name              string          `json:"name"`
	Time              string          `json:"time"`
	CollectionAddress string          `json:"collectionAddress"`
	TypeData          getgemsTypeData `json:"typeData"`
}

type getgemsTypeData struct {
	Type      string `json:"type"`
	Price     string `json:"price"`
	PriceNano string `json:"priceNano"`
	Currency  string `json:"currency"`
}

func (g *Getgems) CheckAuth(ctx context.Context) error {
	_, err := g.get(ctx, "/gifts/collections", url.Values{"limit": {"1"}})
	return err
}

func (g *Getgems) List(ctx context.Context, watch marketport.WatchItem) ([]marketport.Listing, error) {
	if strings.TrimSpace(watch.Collection) == "" {
		return nil, fmt.Errorf("market/getgems: empty collection")
	}
	addr, err := g.resolveCollection(ctx, watch.Collection)
	if err != nil {
		return nil, err
	}
	raw, err := g.listOnSaleCached(ctx, addr)
	if err != nil {
		return nil, err
	}
	wantColl := giftid.Fold(watch.Collection)
	wantModel := giftid.Fold(watch.Model)
	wantBG := giftid.Fold(watch.Backdrop)
	out := make([]marketport.Listing, 0, len(raw))
	for _, n := range raw {
		lot, ok := normalizeGetgemsNFT(n, watch.Collection)
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
	}
	return marketport.TakeCheapest(out, g.listLimit), nil
}

func (g *Getgems) RecentSales(ctx context.Context, like marketport.Listing, limit int) ([]marketport.Sale, error) {
	if limit <= 0 {
		limit = marketport.DefaultSaleLimit
	}
	addr, err := g.resolveCollection(ctx, like.Collection)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("limit", strconv.Itoa(getgemsPageSize))
	q.Set("types", "sold")
	var out []marketport.Sale
	cursor := ""
	wantModel := giftid.Fold(like.Model)
	wantBG := giftid.Fold(like.Backdrop)
	for page := 0; page < g.maxPages && len(out) < limit; page++ {
		if cursor != "" {
			q.Set("after", cursor)
		}
		raw, err := g.get(ctx, "/collection/history/"+url.PathEscape(addr), q)
		if err != nil {
			return nil, err
		}
		var pageData getgemsHistoryPage
		if err := json.Unmarshal(raw, &pageData); err != nil {
			return nil, fmt.Errorf("market/getgems: decode history: %w", err)
		}
		if len(pageData.Items) == 0 {
			break
		}
		addrs := make([]string, 0, len(pageData.Items))
		for _, el := range pageData.Items {
			if a := strings.TrimSpace(el.Address); a != "" {
				addrs = append(addrs, a)
			}
		}
		hydrated, _ := g.nftsByAddress(ctx, addrs)
		for _, el := range pageData.Items {
			sale, ok := normalizeGetgemsSale(el, like.Collection)
			if !ok {
				continue
			}
			if n, ok := hydrated[strings.TrimSpace(el.Address)]; ok {
				model, backdrop, _ := attrsFromGetgems(n.Attributes)
				sale.Model = model
				sale.Backdrop = backdrop
			}
			// History часто без model/backdrop — тогда берём как collection-wide comps.
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
		next := strings.TrimSpace(pageData.Cursor)
		if next == "" || next == cursor {
			break
		}
		cursor = next
	}
	return out, nil
}

func (g *Getgems) listOnSaleCached(ctx context.Context, colAddr string) ([]getgemsNFT, error) {
	g.listMu.Lock()
	if g.listCache != nil {
		if e, ok := g.listCache[colAddr]; ok && time.Since(e.at) < getgemsListCacheTTL {
			out := append([]getgemsNFT(nil), e.items...)
			g.listMu.Unlock()
			return out, nil
		}
	}
	g.listMu.Unlock()

	items, err := g.listOnSale(ctx, colAddr)
	if err != nil {
		return nil, err
	}
	g.listMu.Lock()
	if g.listCache == nil {
		g.listCache = map[string]getgemsListCacheEntry{}
	}
	g.listCache[colAddr] = getgemsListCacheEntry{
		at:    time.Now(),
		items: append([]getgemsNFT(nil), items...),
	}
	g.listMu.Unlock()
	return items, nil
}

func (g *Getgems) listOnSale(ctx context.Context, colAddr string) ([]getgemsNFT, error) {
	seen := map[string]struct{}{}
	var out []getgemsNFT
	for _, path := range []string{
		"/nfts/offchain/on-sale/" + url.PathEscape(colAddr),
		"/nfts/on-sale/" + url.PathEscape(colAddr),
	} {
		cursor := ""
		q := url.Values{}
		q.Set("limit", strconv.Itoa(getgemsPageSize))
		for page := 0; page < g.maxPages; page++ {
			if cursor != "" {
				q.Set("after", cursor)
			}
			raw, err := g.get(ctx, path, q)
			if err != nil {
				if errors.Is(err, errGetgemsNotFound) {
					break
				}
				if len(out) > 0 {
					break
				}
				return nil, err
			}
			var pg getgemsItemsPage
			if err := json.Unmarshal(raw, &pg); err != nil {
				return nil, fmt.Errorf("market/getgems: decode on-sale: %w", err)
			}
			if len(pg.Items) == 0 {
				break
			}
			for _, n := range pg.Items {
				id := strings.TrimSpace(n.Address)
				if id == "" {
					continue
				}
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				out = append(out, n)
			}
			if len(out) >= g.listLimit {
				return out, nil
			}
			next := strings.TrimSpace(pg.Cursor)
			if next == "" || next == cursor {
				break
			}
			cursor = next
		}
	}
	return out, nil
}

func (g *Getgems) resolveCollection(ctx context.Context, name string) (string, error) {
	fold := giftid.Fold(name)
	g.colMu.Lock()
	if g.colByFold != nil && time.Since(g.colFetched) < getgemsColCacheTTL {
		if addr, ok := g.colByFold[fold]; ok {
			g.colMu.Unlock()
			return addr, nil
		}
	}
	g.colMu.Unlock()
	m, err := g.fetchCollections(ctx)
	if err != nil {
		return "", err
	}
	addr, ok := m[fold]
	if !ok {
		return "", fmt.Errorf("market/getgems: unknown collection %q", name)
	}
	return addr, nil
}

func (g *Getgems) fetchCollections(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	cursor := ""
	q := url.Values{}
	q.Set("limit", strconv.Itoa(getgemsPageSize))
	for page := 0; page < g.maxPages; page++ {
		if cursor != "" {
			q.Set("after", cursor)
		}
		raw, err := g.get(ctx, "/gifts/collections", q)
		if err != nil {
			return nil, err
		}
		var pg struct {
			Cursor string `json:"cursor"`
			Items  []struct {
				Address string `json:"address"`
				Name    string `json:"name"`
			} `json:"items"`
		}
		if err := json.Unmarshal(raw, &pg); err != nil {
			return nil, fmt.Errorf("market/getgems: decode collections: %w", err)
		}
		if len(pg.Items) == 0 {
			break
		}
		for _, it := range pg.Items {
			if it.Address == "" || it.Name == "" {
				continue
			}
			out[giftid.Fold(it.Name)] = it.Address
		}
		next := strings.TrimSpace(pg.Cursor)
		if next == "" || next == cursor {
			break
		}
		cursor = next
	}
	g.colMu.Lock()
	g.colByFold = out
	g.colFetched = time.Now()
	g.colMu.Unlock()
	return out, nil
}

func (g *Getgems) nftsByAddress(ctx context.Context, addrs []string) (map[string]getgemsNFT, error) {
	seen := map[string]struct{}{}
	var uniq []string
	for _, a := range addrs {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		uniq = append(uniq, a)
	}
	if len(uniq) == 0 {
		return nil, nil
	}
	out := map[string]getgemsNFT{}
	const batch = 100
	for i := 0; i < len(uniq); i += batch {
		end := i + batch
		if end > len(uniq) {
			end = len(uniq)
		}
		raw, err := g.post(ctx, "/nfts/list", map[string]any{"addressList": uniq[i:end]})
		if err != nil {
			return out, err
		}
		var pg getgemsItemsPage
		if err := json.Unmarshal(raw, &pg); err != nil {
			var items []getgemsNFT
			if err2 := json.Unmarshal(raw, &items); err2 != nil {
				return out, fmt.Errorf("market/getgems: decode nfts/list: %w", err)
			}
			pg.Items = items
		}
		for _, n := range pg.Items {
			id := strings.TrimSpace(n.Address)
			if id != "" {
				out[id] = n
			}
		}
	}
	return out, nil
}

func (g *Getgems) get(ctx context.Context, path string, q url.Values) (json.RawMessage, error) {
	return g.do(ctx, http.MethodGet, path, q, nil)
}

func (g *Getgems) post(ctx context.Context, path string, payload any) (json.RawMessage, error) {
	return g.do(ctx, http.MethodPost, path, nil, payload)
}

func (g *Getgems) do(ctx context.Context, method, path string, q url.Values, payload any) (json.RawMessage, error) {
	apiKey := g.APIKey()
	if apiKey == "" {
		return nil, ErrEmptyToken
	}
	if err := g.gate.wait(ctx); err != nil {
		return nil, err
	}
	u := g.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var bodyReader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", apiKey)
	req.Header.Set("User-Agent", "order-bot/1.0")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("market/getgems: http: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("market/getgems: read: %w", err)
	}
	if res.StatusCode == http.StatusUnauthorized {
		return nil, &UnauthorizedError{Cause: fmt.Errorf("status %d: %s", res.StatusCode, truncate(body, 200))}
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, errGetgemsNotFound
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("market/getgems: status %d: %s", res.StatusCode, truncate(body, 200))
	}
	var env getgemsEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("market/getgems: decode: %w", err)
	}
	if !env.Success {
		return nil, fmt.Errorf("market/getgems: success=false: %s", truncate(body, 200))
	}
	return env.Response, nil
}

func normalizeGetgemsNFT(n getgemsNFT, fallback string) (marketport.Listing, bool) {
	if n.Sale == nil || !strings.EqualFold(n.Sale.Type, "FixPriceSale") {
		return marketport.Listing{}, false
	}
	if n.Sale.Currency != "" && !strings.EqualFold(n.Sale.Currency, "TON") {
		return marketport.Listing{}, false
	}
	price := nanoStringToTON(n.Sale.FullPrice)
	if price <= 0 {
		return marketport.Listing{}, false
	}
	id := strings.TrimSpace(n.Address)
	if id == "" {
		return marketport.Listing{}, false
	}
	coll := firstNonEmpty(n.CollectionName, "")
	if n.Collection != nil {
		coll = firstNonEmpty(coll, n.Collection.Name)
	}
	num := (*int)(nil)
	parsedColl, parsedNum := parseGiftTitle(n.Name)
	if coll == "" {
		coll = parsedColl
	}
	if parsedNum != nil {
		num = parsedNum
	} else if n.Index > 0 {
		idx := n.Index
		num = &idx
	}
	if coll == "" {
		coll = fallback
	}
	model, backdrop, symbol := attrsFromGetgems(n.Attributes)
	return marketport.Listing{
		ID:         id,
		Collection: coll,
		Model:      model,
		Price:      price,
		Number:     num,
		Backdrop:   backdrop,
		Symbol:     symbol,
		URL:        "https://getgems.io/nft/" + id,
		Market:     marketport.MarketGetgems,
	}, true
}

func normalizeGetgemsSale(el getgemsHistoryEl, fallback string) (marketport.Sale, bool) {
	if !strings.EqualFold(el.TypeData.Type, "sold") {
		return marketport.Sale{}, false
	}
	if el.TypeData.Currency != "" && !strings.EqualFold(el.TypeData.Currency, "TON") {
		return marketport.Sale{}, false
	}
	price := nanoStringToTON(el.TypeData.PriceNano)
	if price <= 0 {
		if p, err := strconv.ParseFloat(el.TypeData.Price, 64); err == nil {
			price = p
		}
	}
	if price <= 0 {
		return marketport.Sale{}, false
	}
	coll, num := parseGiftTitle(el.Name)
	if coll == "" {
		coll = fallback
	}
	at, _ := time.Parse(time.RFC3339, el.Time)
	return marketport.Sale{
		Price:      price,
		Number:     num,
		At:         at,
		Collection: coll,
	}, true
}

func attrsFromGetgems(attrs []getgemsAttr) (model, backdrop, symbol string) {
	for _, a := range attrs {
		switch strings.ToLower(strings.TrimSpace(a.TraitType)) {
		case "model":
			model = a.Value
		case "backdrop", "background":
			backdrop = a.Value
		case "symbol":
			symbol = a.Value
		}
	}
	return
}

func parseGiftTitle(name string) (collection string, number *int) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	i := strings.LastIndex(name, "#")
	if i <= 0 {
		return name, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(name[i+1:]))
	if err != nil {
		return name, nil
	}
	collection = strings.TrimSpace(name[:i])
	number = &n
	return collection, number
}

func nanoStringToTON(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil || n <= 0 {
		return 0
	}
	if n > 1e6 {
		return n / 1e9
	}
	return n
}

// ProbeGetgems — ключ Read API жив.
func ProbeGetgems(ctx context.Context, apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return ErrEmptyToken
	}
	return NewGetgems(GetgemsConfig{APIKey: apiKey}).CheckAuth(ctx)
}
