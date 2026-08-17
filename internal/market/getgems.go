package market

import (
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

const (
	defaultGetgemsGraphQL = "https://getgems.io/graphql/"
	getgemsMaxPages       = 3
	getgemsPageSize       = 35 // GraphQL alphaNftItemSearch first: max 35
	getgemsColCacheTTL    = 30 * time.Minute
	getgemsListCacheTTL   = time.Second
	getgemsCatalogPages   = 20

	// Apollo APQ hashes: sha256(print(addTypenameToDocument(query))) from Getgems frontend.
	gqlHashNftSearchInstantSell      = "8ae83a1f1f0c5d89da8ad1535505d91105b73572827d08d368067eac4fcf6346"
	gqlHashCollectionSearch          = "3fd06be62db13d0989dc456b2986fc6c4088b389867bda3f186d3227a3041bab"
	gqlHashGetNftCollectionByAddr    = "010160c8c075e3e3d5e46105b03a31b36344cc7e912ca922e1d16a9467e7980e"
	gqlHashHistoryCollectionNftItems = "d14d1e21fd84908c93c118a231fa723ba753f6ba3fd59a9ff364fbe6362eefce"
	gqlHashGetNftByAddress           = "263b0de8a04fa6de72c9982b791f9f8933b44741b6b0d03b9dd4f4ba090b8b73"

	gqlTelegramGiftsAddr = "EQAbfjxb1uxz66R_c6sjdYysf7kuERaRAvcDXIYfSHWTRwuz"
	gqlSortFixPriceAsc   = `[{"fixPrice":{"order":"asc"}},{"index":{"order":"asc"}}]`
)

var errGetgemsUnknownCollection = errors.New("market/getgems: unknown collection")

// GetgemsConfig — фронтовый GraphQL Getgems (без Read API key).
type GetgemsConfig struct {
	BaseURL   string
	APIKey    string // unused; listings go through public GraphQL
	HTTP      *http.Client
	MaxPages  int
	ListLimit int
}

// Getgems — ридер Telegram Gifts на Getgems через persisted GraphQL.
type Getgems struct {
	baseURL   string
	http      *http.Client
	maxPages  int
	listLimit int
	gate      getgemsGate

	keyMu  sync.RWMutex
	apiKey string

	colMu      sync.Mutex
	colCache   []getgemsNamedCol
	colFetched time.Time

	listMu    sync.Mutex
	listCache map[string]getgemsListCacheEntry
}

func NewGetgems(cfg GetgemsConfig) *Getgems {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = strings.TrimRight(defaultGetgemsGraphQL, "/")
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

// SetAPIKey сохранён для Mini App hook; GraphQL его не использует.
func (g *Getgems) SetAPIKey(key string) {
	if g == nil {
		return
	}
	g.keyMu.Lock()
	g.apiKey = strings.TrimSpace(key)
	g.keyMu.Unlock()
}

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

func (g *Getgems) Enabled() bool { return g != nil }

type getgemsListCacheEntry struct {
	at    time.Time
	items []getgemsNFT
}

type getgemsNFT struct {
	Address           string
	Index             int
	Name              string
	CollectionAddress string
	CollectionName    string
	Attributes        []getgemsAttr
	Sale              *getgemsSale
}

type getgemsAttr struct {
	TraitType string `json:"traitType"`
	Value     string `json:"value"`
}

type getgemsSale struct {
	Type      string
	FullPrice string
	Currency  string
}

func (g *Getgems) CheckAuth(ctx context.Context) error {
	var data struct {
		NFTCollectionByAddress *struct {
			Address string `json:"address"`
			Name    string `json:"name"`
			Type    string `json:"type"`
		} `json:"nftCollectionByAddress"`
	}
	if err := g.gql(ctx, "getNftCollectionByAddress", gqlHashGetNftCollectionByAddr, map[string]any{
		"address": gqlTelegramGiftsAddr,
	}, &data); err != nil {
		return err
	}
	if data.NFTCollectionByAddress == nil || strings.TrimSpace(data.NFTCollectionByAddress.Address) == "" {
		return fmt.Errorf("market/getgems: empty graphql collection")
	}
	return nil
}

func (g *Getgems) List(ctx context.Context, watch marketport.WatchItem) ([]marketport.Listing, error) {
	if strings.TrimSpace(watch.Collection) == "" {
		return nil, fmt.Errorf("market/getgems: empty collection")
	}
	addr, err := g.resolveCollection(ctx, watch.Collection)
	if errors.Is(err, errGetgemsUnknownCollection) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	raw, err := g.listOnSaleCached(ctx, addr, watch.Model, watch.Backdrop)
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
		if wantColl != "" && giftid.Fold(lot.Collection) != wantColl && !giftid.SameCollection(lot.Collection, watch.Collection) && !getgemsNameClose(lot.Collection, watch.Collection) {
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
	if errors.Is(err, errGetgemsUnknownCollection) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	wantModel := giftid.Fold(like.Model)
	wantBG := giftid.Fold(like.Backdrop)
	needHydrate := wantModel != "" || wantBG != ""
	histLimit := limit * 3
	if histLimit < 10 {
		histLimit = 10
	}
	if histLimit > getgemsPageSize {
		histLimit = getgemsPageSize
	}
	var out []marketport.Sale
	cursor := ""
	for page := 0; page < g.maxPages && len(out) < limit; page++ {
		vars := map[string]any{
			"collectionAddress": addr,
			"count":             histLimit,
			"types":             []string{"Sold"},
		}
		if cursor != "" {
			vars["cursor"] = cursor
		}
		var data struct {
			History *struct {
				Cursor string         `json:"cursor"`
				Items  []gqlHistoryEl `json:"items"`
			} `json:"historyCollectionNftItems"`
		}
		if err := g.gql(ctx, "historyCollectionNftItems", gqlHashHistoryCollectionNftItems, vars, &data); err != nil {
			return nil, err
		}
		if data.History == nil || len(data.History.Items) == 0 {
			break
		}
		for _, el := range data.History.Items {
			sale, ok := normalizeGetgemsHistory(el, like.Collection)
			if !ok {
				continue
			}
			if needHydrate {
				if n, err := g.nftByAddress(ctx, strings.TrimSpace(el.Address)); err == nil {
					model, backdrop, _ := attrsFromGetgems(n.Attributes)
					sale.Model = model
					sale.Backdrop = backdrop
				}
			}
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
		next := strings.TrimSpace(data.History.Cursor)
		if next == "" || next == cursor {
			break
		}
		cursor = next
	}
	return out, nil
}

func (g *Getgems) listOnSaleCached(ctx context.Context, colAddr, model, backdrop string) ([]getgemsNFT, error) {
	key := colAddr + "\x00" + giftid.Fold(model) + "\x00" + giftid.Fold(backdrop)
	g.listMu.Lock()
	if g.listCache != nil {
		if e, ok := g.listCache[key]; ok && time.Since(e.at) < getgemsListCacheTTL {
			out := append([]getgemsNFT(nil), e.items...)
			g.listMu.Unlock()
			return out, nil
		}
	}
	g.listMu.Unlock()

	items, err := g.listOnSale(ctx, colAddr, model, backdrop)
	if err != nil {
		return nil, err
	}
	g.listMu.Lock()
	if g.listCache == nil {
		g.listCache = map[string]getgemsListCacheEntry{}
	}
	g.listCache[key] = getgemsListCacheEntry{at: time.Now(), items: append([]getgemsNFT(nil), items...)}
	g.listMu.Unlock()
	return items, nil
}

func (g *Getgems) listOnSale(ctx context.Context, colAddr, model, backdrop string) ([]getgemsNFT, error) {
	queryObj := map[string]any{
		"$and": []any{
			map[string]any{"collectionAddressList": []string{colAddr}},
			map[string]any{"saleType": "fix_price"},
		},
	}
	queryRaw, err := json.Marshal(queryObj)
	if err != nil {
		return nil, err
	}
	vars := map[string]any{
		"count": getgemsPageSize,
		"query": string(queryRaw),
		"sort":  gqlSortFixPriceAsc,
	}
	if attrs := getgemsAttributesJSON(model, backdrop); attrs != "" {
		vars["attributes"] = attrs
	}

	seen := map[string]struct{}{}
	var out []getgemsNFT
	cursor := ""
	for page := 0; page < g.maxPages; page++ {
		if cursor != "" {
			vars["cursor"] = cursor
		}
		var data struct {
			Search *struct {
				Edges []struct {
					Cursor string     `json:"cursor"`
					Node   gqlNftNode `json:"node"`
				} `json:"edges"`
				Info struct {
					HasNextPage bool `json:"hasNextPage"`
				} `json:"info"`
			} `json:"alphaNftItemSearch"`
		}
		if err := g.gql(ctx, "nftSearchInstantSell", gqlHashNftSearchInstantSell, vars, &data); err != nil {
			return nil, err
		}
		if data.Search == nil || len(data.Search.Edges) == 0 {
			break
		}
		var lastCursor string
		for _, e := range data.Search.Edges {
			n := nftFromGQL(e.Node)
			id := strings.TrimSpace(n.Address)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, n)
			lastCursor = strings.TrimSpace(e.Cursor)
		}
		if len(out) >= g.listLimit {
			return out, nil
		}
		if !data.Search.Info.HasNextPage || lastCursor == "" || lastCursor == cursor {
			break
		}
		cursor = lastCursor
	}
	return out, nil
}

type getgemsNamedCol struct {
	Name    string
	Address string
}

func (g *Getgems) resolveCollection(ctx context.Context, name string) (string, error) {
	cols, err := g.giftCollections(ctx)
	if err != nil {
		return "", err
	}
	bestAddr := ""
	bestScore := 0
	for _, c := range cols {
		sc := getgemsCollectionScore(name, c.Name)
		if sc > bestScore {
			bestScore = sc
			bestAddr = c.Address
		}
	}
	if bestScore < 2 || bestAddr == "" {
		return "", fmt.Errorf("%w %q", errGetgemsUnknownCollection, name)
	}
	return bestAddr, nil
}

func (g *Getgems) giftCollections(ctx context.Context) ([]getgemsNamedCol, error) {
	g.colMu.Lock()
	if len(g.colCache) > 0 && time.Since(g.colFetched) < getgemsColCacheTTL {
		out := append([]getgemsNamedCol(nil), g.colCache...)
		g.colMu.Unlock()
		return out, nil
	}
	g.colMu.Unlock()

	queryObj := map[string]any{
		"$and": []any{
			map[string]any{"collectionAddressList": []string{gqlTelegramGiftsAddr}},
		},
	}
	queryRaw, err := json.Marshal(queryObj)
	if err != nil {
		return nil, err
	}
	var cols []getgemsNamedCol
	cursor := ""
	for page := 0; page < getgemsCatalogPages; page++ {
		vars := map[string]any{
			"count": getgemsPageSize,
			"query": string(queryRaw),
		}
		if cursor != "" {
			vars["cursor"] = cursor
		}
		var data struct {
			Search *struct {
				Edges []struct {
					Cursor string `json:"cursor"`
					Node   struct {
						Address string `json:"address"`
						Name    string `json:"name"`
					} `json:"node"`
				} `json:"edges"`
				Info struct {
					HasNextPage bool `json:"hasNextPage"`
				} `json:"info"`
			} `json:"alphaNftCollectionSearch"`
		}
		if err := g.gql(ctx, "collectionSearch", gqlHashCollectionSearch, vars, &data); err != nil {
			return nil, err
		}
		if data.Search == nil || len(data.Search.Edges) == 0 {
			break
		}
		var lastCursor string
		for _, e := range data.Search.Edges {
			addr := strings.TrimSpace(e.Node.Address)
			nm := strings.TrimSpace(e.Node.Name)
			if addr == "" || nm == "" {
				continue
			}
			cols = append(cols, getgemsNamedCol{Name: nm, Address: addr})
			lastCursor = strings.TrimSpace(e.Cursor)
		}
		if !data.Search.Info.HasNextPage || lastCursor == "" || lastCursor == cursor {
			break
		}
		cursor = lastCursor
	}
	g.colMu.Lock()
	g.colCache = append([]getgemsNamedCol(nil), cols...)
	g.colFetched = time.Now()
	g.colMu.Unlock()
	return cols, nil
}

func (g *Getgems) nftByAddress(ctx context.Context, addr string) (getgemsNFT, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return getgemsNFT{}, fmt.Errorf("market/getgems: empty nft address")
	}
	var data struct {
		NFT *gqlNftNode `json:"nft"`
	}
	if err := g.gql(ctx, "getNftByAddress", gqlHashGetNftByAddress, map[string]any{
		"address": addr,
	}, &data); err != nil {
		return getgemsNFT{}, err
	}
	if data.NFT == nil {
		return getgemsNFT{}, fmt.Errorf("market/getgems: nft %s not found", addr)
	}
	n := nftFromGQL(*data.NFT)
	if strings.TrimSpace(n.Address) == "" {
		n.Address = addr
	}
	return n, nil
}

type gqlNftNode struct {
	Name       string        `json:"name"`
	Address    string        `json:"address"`
	Index      int           `json:"index"`
	Attributes []getgemsAttr `json:"attributes"`
	Collection *struct {
		Address string `json:"address"`
		Name    string `json:"name"`
		Type    string `json:"type"`
	} `json:"collection"`
	Sale *struct {
		Typename  string `json:"__typename"`
		FullPrice string `json:"fullPrice"`
		Currency  string `json:"currency"`
	} `json:"sale"`
}

type gqlHistoryEl struct {
	Address string          `json:"address"`
	Time    json.RawMessage `json:"time"`
	NFT     *struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	} `json:"nft"`
	TypeData struct {
		Typename string `json:"__typename"`
		Type     string `json:"type"`
		Price    string `json:"price"`
		Currency string `json:"currency"`
	} `json:"typeData"`
}

func nftFromGQL(n gqlNftNode) getgemsNFT {
	out := getgemsNFT{
		Address:    strings.TrimSpace(n.Address),
		Index:      n.Index,
		Name:       n.Name,
		Attributes: n.Attributes,
	}
	if n.Collection != nil {
		out.CollectionAddress = n.Collection.Address
		out.CollectionName = n.Collection.Name
	}
	if n.Sale != nil && (n.Sale.Typename == "NftSaleFixPrice" || strings.EqualFold(n.Sale.Typename, "NftSaleFixPrice")) {
		out.Sale = &getgemsSale{
			Type:      "FixPriceSale",
			FullPrice: n.Sale.FullPrice,
			Currency:  n.Sale.Currency,
		}
	}
	return out
}

func (g *Getgems) gql(ctx context.Context, operation, hash string, variables map[string]any, dest any) error {
	if err := g.gate.wait(ctx); err != nil {
		return err
	}
	varsJSON, err := json.Marshal(variables)
	if err != nil {
		return err
	}
	extJSON, err := json.Marshal(map[string]any{
		"clientLibrary":  map[string]any{"name": "@apollo/client", "version": "4.1.9"},
		"persistedQuery": map[string]any{"version": 1, "sha256Hash": hash},
	})
	if err != nil {
		return err
	}
	q := url.Values{}
	q.Set("operationName", operation)
	q.Set("variables", string(varsJSON))
	q.Set("extensions", string(extJSON))
	u := g.baseURL + "/?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://getgems.io")
	req.Header.Set("Referer", "https://getgems.io/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1")
	req.Header.Set("x-apollo-operation-name", operation)
	req.Header.Set("x-gg-frontend", "1")

	res, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("market/getgems: http: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("market/getgems: read: %w", err)
	}
	if res.StatusCode == http.StatusUnauthorized {
		return &UnauthorizedError{Cause: fmt.Errorf("status %d: %s", res.StatusCode, truncate(body, 200))}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("market/getgems: status %d: %s", res.StatusCode, truncate(body, 200))
	}
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("market/getgems: decode: %w", err)
	}
	if len(env.Errors) > 0 && (len(env.Data) == 0 || string(env.Data) == "null") {
		return fmt.Errorf("market/getgems: graphql: %s", env.Errors[0].Message)
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		if len(env.Errors) > 0 {
			return fmt.Errorf("market/getgems: graphql: %s", env.Errors[0].Message)
		}
		return fmt.Errorf("market/getgems: empty graphql data")
	}
	if err := json.Unmarshal(env.Data, dest); err != nil {
		return fmt.Errorf("market/getgems: decode data: %w", err)
	}
	return nil
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
	coll := strings.TrimSpace(n.CollectionName)
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

func normalizeGetgemsHistory(el gqlHistoryEl, fallback string) (marketport.Sale, bool) {
	if !strings.EqualFold(el.TypeData.Type, "sold") && el.TypeData.Typename != "HistoryTypeSold" {
		return marketport.Sale{}, false
	}
	if el.TypeData.Currency != "" && !strings.EqualFold(el.TypeData.Currency, "TON") {
		return marketport.Sale{}, false
	}
	price := nanoStringToTON(el.TypeData.Price)
	if price <= 0 {
		return marketport.Sale{}, false
	}
	name := ""
	if el.NFT != nil {
		name = el.NFT.Name
	}
	coll, num := parseGiftTitle(name)
	if coll == "" {
		coll = fallback
	}
	return marketport.Sale{
		Price:      price,
		Number:     num,
		At:         parseGetgemsTime(el.Time),
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

func parseGetgemsTime(raw json.RawMessage) time.Time {
	s := strings.TrimSpace(string(raw))
	s = strings.Trim(s, `"`)
	if s == "" || s == "null" {
		return time.Time{}
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
		if n > 1e12 {
			return time.UnixMilli(n)
		}
		return time.Unix(n, 0)
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func getgemsAttributesJSON(model, backdrop string) string {
	var pairs [][]any
	if m := strings.TrimSpace(model); m != "" {
		pairs = append(pairs, []any{"Model", []string{m}})
	}
	if b := strings.TrimSpace(backdrop); b != "" {
		pairs = append(pairs, []any{"Backdrop", []string{b}})
	}
	if len(pairs) == 0 {
		return ""
	}
	raw, err := json.Marshal(pairs)
	if err != nil {
		return ""
	}
	return string(raw)
}

func getgemsCollectionScore(want, name string) int {
	if giftid.Fold(want) == giftid.Fold(name) {
		return 3
	}
	if getgemsNameClose(want, name) {
		return 2
	}
	wf, nf := giftid.Fold(want), giftid.Fold(name)
	if wf != "" && (strings.HasPrefix(nf, wf) || strings.HasPrefix(wf, nf)) {
		return 1
	}
	return 0
}

func getgemsNameClose(a, b string) bool {
	af, bf := giftid.Fold(a), giftid.Fold(b)
	if af == bf {
		return true
	}
	return af+"s" == bf || bf+"s" == af
}

// ProbeGetgems — фронтовый GraphQL отвечает (Read API key не нужен).
func ProbeGetgems(ctx context.Context, _ string) error {
	return NewGetgems(GetgemsConfig{}).CheckAuth(ctx)
}
