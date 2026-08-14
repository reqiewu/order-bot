package market

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/reqiewu/order-bot/internal/giftid"
	"github.com/reqiewu/order-bot/internal/marketport"
)

const (
	defaultBaseURL = "https://api.tgmrkt.io"
	// defaultMaxPages — safety-cap; обычно останавливаемся раньше по ListLimit.
	defaultMaxPages = 20
	pageSize        = 20
)

// Market — HTTP-адаптерный алиас порта чтения (типы и контракт — в marketport).
type Market = marketport.MarketReader

// Config — настройки HTTP-клиента MRKT.
type Config struct {
	BaseURL   string
	Auth      TokenProvider
	HTTP      *http.Client
	MaxPages  int
	ListLimit int // топ N дешёвых; 0 = DefaultListLimit
}

// MRKT — неофициальный HTTP-адаптер маркета MRKT.
type MRKT struct {
	baseURL   string
	auth      TokenProvider
	http      *http.Client
	maxPages  int
	listLimit int
	gate      mrktGate
}

// NewMRKT создаёт клиент Market для MRKT.
func NewMRKT(cfg Config) *MRKT {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = defaultBaseURL
	}
	httpClient := cfg.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	maxPages := cfg.MaxPages
	if maxPages <= 0 {
		maxPages = defaultMaxPages
	}
	listLimit := cfg.ListLimit
	if listLimit <= 0 {
		listLimit = marketport.DefaultListLimit
	}
	return &MRKT{
		baseURL:   base,
		auth:      cfg.Auth,
		http:      httpClient,
		maxPages:  maxPages,
		listLimit: listLimit,
	}
}

func (m *MRKT) Enabled() bool { return m != nil && tokenPresent(m.auth) }

// Ensure MRKT satisfies the inner MarketReader port.
var _ marketport.MarketReader = (*MRKT)(nil)

type salingRequest struct {
	CollectionNames []string `json:"collectionNames"`
	ModelNames      []string `json:"modelNames"`
	BackdropNames   []string `json:"backdropNames"`
	SymbolNames     []string `json:"symbolNames"`
	Ordering        string   `json:"ordering"`
	LowToHigh       bool     `json:"lowToHigh"`
	MaxPrice        *float64 `json:"maxPrice"`
	MinPrice        *float64 `json:"minPrice"`
	Mintable        *bool    `json:"mintable"`
	Number          *int     `json:"number"`
	Count           int      `json:"count"`
	Cursor          string   `json:"cursor"`
	Query           *string  `json:"query"`
	PromotedFirst   bool     `json:"promotedFirst"`
}

type salingResponse struct {
	Gifts  []rawGift      `json:"gifts"`
	Cursor flexibleCursor `json:"cursor"`
}

// rawGift принимает несколько алиасов полей неофициального API MRKT.
type rawGift struct {
	ID             string          `json:"id"`
	GiftID         string          `json:"gift_id"`
	Name           string          `json:"name"`
	Collection     string          `json:"collection"`
	CollectionName string          `json:"collectionName"`
	Title          string          `json:"title"`
	Model          string          `json:"model"`
	ModelName      string          `json:"modelName"`
	ModelTitle     string          `json:"modelTitle"`
	Backdrop       string          `json:"backdrop"`
	BackdropName   string          `json:"backdropName"`
	Symbol         string          `json:"symbol"`
	SymbolName     string          `json:"symbolName"`
	Number         *int            `json:"number"`
	Price          json.RawMessage `json:"price"`
	PriceTon       float64         `json:"price_ton"`
	SalePrice      float64         `json:"salePrice"`
	URL            string          `json:"url"`
}

type flexibleCursor string

func (c *flexibleCursor) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*c = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*c = flexibleCursor(s)
		return nil
	}
	*c = ""
	return nil
}

// List загружает ограниченное число страниц лотов для элемента Radar-фильтра.
func (m *MRKT) List(ctx context.Context, watch marketport.WatchItem) ([]marketport.Listing, error) {
	if m.auth == nil {
		return nil, fmt.Errorf("market: auth provider is required")
	}

	token, err := m.auth.Token(ctx)
	if err != nil {
		return nil, err
	}

	names := giftid.MRKTNames(giftid.Identity{
		Collection: watch.Collection,
		Model:      watch.Model,
		Backdrop:   watch.Backdrop,
	})
	models := []string{}
	if names.Model != "" {
		models = []string{names.Model}
	}
	backdrops := []string{}
	if names.Backdrop != "" {
		backdrops = []string{names.Backdrop}
	}

	var out []marketport.Listing
	cursor := ""
	for page := 0; page < m.maxPages && len(out) < m.listLimit; page++ {
		reqBody := salingRequest{
			CollectionNames: []string{names.Collection},
			ModelNames:      models,
			BackdropNames:   backdrops,
			SymbolNames:     []string{},
			Ordering:        "Price",
			LowToHigh:       true,
			Count:           pageSize,
			Cursor:          cursor,
			PromotedFirst:   false,
		}
		resp, err := m.postSaling(ctx, token, reqBody)
		if err != nil {
			return nil, err
		}
		for _, g := range resp.Gifts {
			listing, ok := normalizeGift(g, names.Collection)
			if !ok {
				continue
			}
			out = append(out, listing)
			if len(out) >= m.listLimit {
				break
			}
		}
		next := string(resp.Cursor)
		if next == "" || next == cursor {
			break
		}
		cursor = next
	}
	return marketport.TakeCheapest(out, m.listLimit), nil
}

func (m *MRKT) postSaling(ctx context.Context, token string, body salingRequest) (*salingResponse, error) {
	if err := m.gate.wait(ctx); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("market: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/api/v1/gifts/saling", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("market: new request: %w", err)
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Cookie", "access_token="+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://cdn.tgmrkt.io")
	req.Header.Set("Referer", "https://cdn.tgmrkt.io/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; order-bot/1.0)")

	res, err := m.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("market: http: %w", err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("market: read body: %w", err)
	}
	if res.StatusCode == http.StatusUnauthorized {
		return nil, &UnauthorizedError{Cause: fmt.Errorf("status %d: %s", res.StatusCode, truncate(raw, 200))}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("market: unexpected status %d: %s", res.StatusCode, truncate(raw, 200))
	}

	var parsed salingResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("market: decode response: %w", err)
	}
	return &parsed, nil
}

func normalizeGift(g rawGift, fallbackCollection string) (marketport.Listing, bool) {
	id := firstNonEmpty(g.ID, g.GiftID, g.Name)
	if id == "" {
		return marketport.Listing{}, false
	}
	collection := firstNonEmpty(g.Collection, g.CollectionName, g.Title, fallbackCollection)
	// Живой MRKT saling отдаёт salePrice в nanoTON; фикстуры могут использовать price_ton в TON.
	price := g.PriceTon
	if price == 0 {
		price = tonFromAPI(g.SalePrice)
	}
	if price == 0 {
		price = parseNanoTON(g.Price)
	}
	url := g.URL
	if url == "" {
		url = "https://t.me/mrkt/app?startapp=" + id
	}
	return marketport.Listing{
		ID:         id,
		Collection: collection,
		Model:      firstNonEmpty(g.Model, g.ModelName, g.ModelTitle),
		Price:      price,
		Number:     g.Number,
		Backdrop:   firstNonEmpty(g.Backdrop, g.BackdropName),
		Symbol:     firstNonEmpty(g.Symbol, g.SymbolName),
		URL:        url,
	}, true
}

// tonFromAPI: значения ≫ 1e6 считаем nanoTON (та же эвристика, что в feed).
func tonFromAPI(n float64) float64 {
	if n <= 0 {
		return 0
	}
	if n > 1e6 {
		return n / nanoTON
	}
	return n
}

func parsePrice(raw json.RawMessage) float64 {
	return parseNanoTON(raw)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
