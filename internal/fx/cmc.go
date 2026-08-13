package fx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// GramCMCID — CoinMarketCap id для Gram (prev. Toncoin).
// https://coinmarketcap.com/currencies/gram/
const GramCMCID = 11419

const defaultCMCQuotesURL = "https://pro-api.coinmarketcap.com/public-api/v3/cryptocurrency/quotes/latest"

// Quote — справочный курс TON→USD (не для триггера спреда).
type Quote struct {
	USDPerTON float64
	Source    string
	At        time.Time
}

// RateSource — кэшируемый курс.
type RateSource interface {
	Rate(ctx context.Context) (Quote, error)
}

// CMC — курс с CoinMarketCap public-api (Gram / TON).
type CMC struct {
	HTTP    *http.Client
	BaseURL string // override for tests
	CoinID  int
	TTL     time.Duration

	mu     sync.Mutex
	cached Quote
}

func NewCMC() *CMC {
	return &CMC{
		HTTP:    &http.Client{Timeout: 8 * time.Second},
		BaseURL: defaultCMCQuotesURL,
		CoinID:  GramCMCID,
		TTL:     45 * time.Second,
	}
}

func (c *CMC) Rate(ctx context.Context) (Quote, error) {
	if c == nil {
		return Quote{}, fmt.Errorf("fx: nil cmc")
	}
	ttl := c.TTL
	if ttl <= 0 {
		ttl = 45 * time.Second
	}
	c.mu.Lock()
	if !c.cached.At.IsZero() && time.Since(c.cached.At) < ttl && c.cached.USDPerTON > 0 {
		q := c.cached
		c.mu.Unlock()
		return q, nil
	}
	c.mu.Unlock()

	q, err := c.fetch(ctx)
	if err != nil {
		c.mu.Lock()
		stale := c.cached
		c.mu.Unlock()
		if stale.USDPerTON > 0 {
			return stale, nil // лучше старый курс, чем пусто
		}
		return Quote{}, err
	}
	c.mu.Lock()
	c.cached = q
	c.mu.Unlock()
	return q, nil
}

func (c *CMC) fetch(ctx context.Context) (Quote, error) {
	base := c.BaseURL
	if base == "" {
		base = defaultCMCQuotesURL
	}
	id := c.CoinID
	if id <= 0 {
		id = GramCMCID
	}
	url := fmt.Sprintf("%s?id=%d&convert=USD", base, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Quote{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "order-bot/1.0")

	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return Quote{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Quote{}, fmt.Errorf("fx: cmc status %s", resp.Status)
	}

	var raw struct {
		Data []struct {
			ID    int `json:"id"`
			Quote []struct {
				Symbol string  `json:"symbol"`
				Price  float64 `json:"price"`
			} `json:"quote"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Quote{}, fmt.Errorf("fx: cmc decode: %w", err)
	}
	if len(raw.Data) == 0 {
		return Quote{}, fmt.Errorf("fx: cmc empty data")
	}
	var price float64
	for _, q := range raw.Data[0].Quote {
		if q.Symbol == "USD" && q.Price > 0 {
			price = q.Price
			break
		}
	}
	if price <= 0 && len(raw.Data[0].Quote) > 0 {
		price = raw.Data[0].Quote[0].Price
	}
	if price <= 0 {
		return Quote{}, fmt.Errorf("fx: cmc no usd price")
	}
	return Quote{USDPerTON: price, Source: "CMC", At: time.Now().UTC()}, nil
}
