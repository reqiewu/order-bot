package market

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/reqiewu/order-bot/internal/marketport"
)

const (
	feedPageSize = 50
	feedMaxPages = 5
	nanoTON      = 1_000_000_000.0
)

// DefaultSaleLimit реэкспорт лимита comps для адаптеров/поллера.
const DefaultSaleLimit = marketport.DefaultSaleLimit

// Sale — алиас доменной продажи (совместимость с тестами адаптера).
type Sale = marketport.Sale

type feedRequest struct {
	Count           int      `json:"count"`
	Cursor          string   `json:"cursor"`
	CollectionNames []string `json:"collectionNames,omitempty"`
	ModelNames      []string `json:"modelNames,omitempty"`
	BackdropNames   []string `json:"backdropNames,omitempty"`
}

type feedResponse struct {
	Items  []feedItem     `json:"items"`
	Cursor flexibleCursor `json:"cursor"`
}

type feedItem struct {
	Type   string          `json:"type"`
	Amount json.RawMessage `json:"amount"`
	Date   string          `json:"date"`
	Gift   feedGift        `json:"gift"`
}

type feedGift struct {
	CollectionName  string `json:"collectionName"`
	CollectionTitle string `json:"collectionTitle"`
	ModelName       string `json:"modelName"`
	ModelTitle      string `json:"modelTitle"`
	BackdropName    string `json:"backdropName"`
	Number          *int   `json:"number"`
}

// RecentSales возвращает до limit недавних sale по модели+фону листинга
// (коллекция / модель / фон уходят в feed-фильтр). Порядок: от старых к новым
// среди выбранных comps (в алерте читается как история → сейчас).
// Пустой результат — валиден (в алерте будет none).
func (m *MRKT) RecentSales(ctx context.Context, like marketport.Listing, limit int) ([]marketport.Sale, error) {
	if m.auth == nil {
		return nil, fmt.Errorf("market: auth provider is required")
	}
	if limit <= 0 {
		limit = DefaultSaleLimit
	}
	token, err := m.auth.Token(ctx)
	if err != nil {
		return nil, err
	}

	var out []Sale
	cursor := ""
	for page := 0; page < feedMaxPages && len(out) < limit; page++ {
		reqBody := feedRequest{
			Count:  feedPageSize,
			Cursor: cursor,
		}
		if like.Collection != "" {
			reqBody.CollectionNames = []string{like.Collection}
		}
		if like.Model != "" {
			reqBody.ModelNames = []string{like.Model}
		}
		if like.Backdrop != "" {
			reqBody.BackdropNames = []string{like.Backdrop}
		}
		resp, err := m.postFeed(ctx, token, reqBody)
		if err != nil {
			return nil, err
		}
		for _, item := range resp.Items {
			if item.Type != "sale" {
				continue
			}
			sale, ok := normalizeFeedSale(item)
			if !ok {
				continue
			}
			if !saleMatchesListing(like, sale) {
				continue
			}
			out = append(out, sale)
			if len(out) >= limit {
				break
			}
		}
		next := string(resp.Cursor)
		if next == "" || next == cursor {
			break
		}
		cursor = next
	}
	// Feed отдаёт новые сверху; в алерте показываем старые → новые.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func saleMatchesListing(like marketport.Listing, sale Sale) bool {
	if like.Collection != "" && sale.Collection != like.Collection {
		return false
	}
	if like.Model != "" && sale.Model != like.Model {
		return false
	}
	// Фон на листинге сужает comps; пустой фон → только модель (+ коллекция).
	if like.Backdrop != "" && sale.Backdrop != like.Backdrop {
		return false
	}
	return true
}

func normalizeFeedSale(item feedItem) (Sale, bool) {
	price := parseNanoTON(item.Amount)
	if price <= 0 {
		return Sale{}, false
	}
	collection := firstNonEmpty(item.Gift.CollectionName, item.Gift.CollectionTitle)
	model := firstNonEmpty(item.Gift.ModelName, item.Gift.ModelTitle)
	backdrop := item.Gift.BackdropName
	at, _ := time.Parse(time.RFC3339Nano, item.Date)
	if at.IsZero() {
		at, _ = time.Parse(time.RFC3339, item.Date)
	}
	return Sale{
		Price:      price,
		Number:     item.Gift.Number,
		At:         at,
		Collection: collection,
		Model:      model,
		Backdrop:   backdrop,
	}, true
}

func parseNanoTON(raw json.RawMessage) float64 {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		if n > 1e6 { // nanoTON
			return n / nanoTON
		}
		return n
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		var parsed float64
		if _, err := fmt.Sscanf(s, "%f", &parsed); err == nil {
			if parsed > 1e6 {
				return parsed / nanoTON
			}
			return parsed
		}
	}
	return 0
}

func (m *MRKT) postFeed(ctx context.Context, token string, body feedRequest) (*feedResponse, error) {
	if err := m.gate.wait(ctx); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("market: marshal feed: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/api/v1/feed", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("market: new feed request: %w", err)
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
		return nil, fmt.Errorf("market: feed http: %w", err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("market: feed read: %w", err)
	}
	if res.StatusCode == http.StatusUnauthorized {
		return nil, &UnauthorizedError{Cause: fmt.Errorf("status %d: %s", res.StatusCode, truncate(raw, 200))}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("market: feed status %d: %s", res.StatusCode, truncate(raw, 200))
	}

	var parsed feedResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("market: feed decode: %w", err)
	}
	return &parsed, nil
}
