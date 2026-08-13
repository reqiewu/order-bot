package market

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

const defaultCatalogLimit = 10

// CatalogSort — порядок списка каталога (после фильтра по Query).
type CatalogSort string

const (
	CatalogSortNone       CatalogSort = ""
	CatalogSortVolumeDesc CatalogSort = "volume_desc"
	CatalogSortFloorAsc   CatalogSort = "floor_asc"
)

// CatalogItem — элемент каталога для подбора фильтра (каноническое имя + подпись).
type CatalogItem struct {
	Name       string
	Title      string
	FloorNano  int64 // floorPriceNanoTons; 0 если нет
	VolumeNano int64 // volume; 0 если нет
}

// CatalogQuery — поиск и постраничный срез каталога.
type CatalogQuery struct {
	Query  string // подстрока по Name/Title, без учёта регистра
	Offset int
	Limit  int // 0 → defaultCatalogLimit
	Sort   CatalogSort
}

// CatalogPage — страница результатов каталога.
type CatalogPage struct {
	Items   []CatalogItem
	Total   int
	HasMore bool
}

// Catalog — подбор коллекций / моделей / фонов с MRKT.
type Catalog interface {
	Collections(ctx context.Context, q CatalogQuery) (CatalogPage, error)
	Models(ctx context.Context, collection string, q CatalogQuery) (CatalogPage, error)
	Backdrops(ctx context.Context, collection, model string, q CatalogQuery) (CatalogPage, error)
}

var _ Catalog = (*MRKT)(nil)

// Collections загружает каталог коллекций, фильтрует по Query и режет по Offset/Limit.
func (m *MRKT) Collections(ctx context.Context, q CatalogQuery) (CatalogPage, error) {
	raw, err := m.doJSON(ctx, http.MethodGet, "/api/v1/gifts/collections", nil)
	if err != nil {
		return CatalogPage{}, err
	}
	items, err := parseCatalogItems(raw, "collections", "collection")
	if err != nil {
		return CatalogPage{}, fmt.Errorf("market: collections: %w", err)
	}
	return paginateCatalog(items, q), nil
}

// Models загружает модели коллекции (и опционально ищет по Query).
func (m *MRKT) Models(ctx context.Context, collection string, q CatalogQuery) (CatalogPage, error) {
	collection = strings.TrimSpace(collection)
	if collection == "" {
		return CatalogPage{}, fmt.Errorf("market: models: collection is required")
	}
	body := map[string]any{
		"collections": []string{collection},
	}
	raw, err := m.doJSON(ctx, http.MethodPost, "/api/v1/gifts/models", body)
	if err != nil {
		return CatalogPage{}, err
	}
	items, err := parseCatalogItems(raw, "models", "model")
	if err != nil {
		return CatalogPage{}, fmt.Errorf("market: models: %w", err)
	}
	return paginateCatalog(items, q), nil
}

// Backdrops загружает фоны для коллекции (и модели, если задана).
func (m *MRKT) Backdrops(ctx context.Context, collection, model string, q CatalogQuery) (CatalogPage, error) {
	collection = strings.TrimSpace(collection)
	if collection == "" {
		return CatalogPage{}, fmt.Errorf("market: backdrops: collection is required")
	}
	body := map[string]any{
		"collections": []string{collection},
		"models":      []string{},
	}
	if model = strings.TrimSpace(model); model != "" {
		body["models"] = []string{model}
	}
	raw, err := m.doJSON(ctx, http.MethodPost, "/api/v1/gifts/backdrops", body)
	if err != nil {
		return CatalogPage{}, err
	}
	items, err := parseCatalogItems(raw, "backdrops", "backdrop")
	if err != nil {
		return CatalogPage{}, fmt.Errorf("market: backdrops: %w", err)
	}
	return paginateCatalog(items, q), nil
}

func paginateCatalog(items []CatalogItem, q CatalogQuery) CatalogPage {
	limit := q.Limit
	if limit <= 0 {
		limit = defaultCatalogLimit
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}
	filtered := filterCatalog(items, q.Query)
	filtered = sortCatalog(filtered, q.Sort)
	total := len(filtered)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := filtered[offset:end]
	return CatalogPage{
		Items:   page,
		Total:   total,
		HasMore: end < total,
	}
}

func filterCatalog(items []CatalogItem, query string) []CatalogItem {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		out := make([]CatalogItem, len(items))
		copy(out, items)
		return out
	}
	var out []CatalogItem
	for _, it := range items {
		name := strings.ToLower(it.Name)
		title := strings.ToLower(it.Title)
		if strings.Contains(name, query) || strings.Contains(title, query) {
			out = append(out, it)
		}
	}
	return out
}

func sortCatalog(items []CatalogItem, mode CatalogSort) []CatalogItem {
	if mode == CatalogSortNone || len(items) < 2 {
		return items
	}
	out := make([]CatalogItem, len(items))
	copy(out, items)
	switch mode {
	case CatalogSortVolumeDesc:
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].VolumeNano != out[j].VolumeNano {
				return out[i].VolumeNano > out[j].VolumeNano
			}
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		})
	case CatalogSortFloorAsc:
		sort.SliceStable(out, func(i, j int) bool {
			// 0 floor (unknown) в конец
			ai, aj := out[i].FloorNano, out[j].FloorNano
			if ai == 0 && aj == 0 {
				return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
			}
			if ai == 0 {
				return false
			}
			if aj == 0 {
				return true
			}
			if ai != aj {
				return ai < aj
			}
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		})
	}
	return out
}

func parseCatalogItems(raw []byte, wrapKeys ...string) ([]CatalogItem, error) {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	// Прямой массив объектов или строк.
	if raw[0] == '[' {
		return decodeCatalogArray(raw)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	for _, key := range wrapKeys {
		if v, ok := obj[key]; ok {
			return decodeCatalogArray(v)
		}
	}
	// Частый алиас.
	for _, key := range []string{"items", "data", "result"} {
		if v, ok := obj[key]; ok {
			return decodeCatalogArray(v)
		}
	}
	return nil, fmt.Errorf("unexpected catalog JSON shape")
}

func decodeCatalogArray(raw []byte) ([]CatalogItem, error) {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	// Массив строк.
	var names []string
	if err := json.Unmarshal(raw, &names); err == nil {
		out := make([]CatalogItem, 0, len(names))
		for _, n := range names {
			n = strings.TrimSpace(n)
			if n == "" {
				continue
			}
			out = append(out, CatalogItem{Name: n, Title: n})
		}
		return out, nil
	}

	var objs []struct {
		Name  string `json:"name"`
		Title string `json:"title"`
		// Алиасы из разных ответов API.
		CollectionName  string `json:"collectionName"`
		CollectionTitle string `json:"collectionTitle"`
		ModelName       string `json:"modelName"`
		ModelTitle      string `json:"modelTitle"`
		BackdropName    string `json:"backdropName"`
		BackdropTitle   string `json:"backdropTitle"`
		FloorPriceNano  *int64 `json:"floorPriceNanoTons"`
		Volume          *int64 `json:"volume"`
	}
	if err := json.Unmarshal(raw, &objs); err != nil {
		return nil, err
	}
	out := make([]CatalogItem, 0, len(objs))
	for _, o := range objs {
		// Модели/фоны приходят вместе с collectionName — берём более узкое поле.
		name := firstNonEmpty(o.Name, o.ModelName, o.BackdropName, o.CollectionName)
		if name == "" {
			continue
		}
		title := firstNonEmpty(o.Title, o.ModelTitle, o.BackdropTitle, o.CollectionTitle, name)
		it := CatalogItem{Name: name, Title: title}
		if o.FloorPriceNano != nil {
			it.FloorNano = *o.FloorPriceNano
		}
		if o.Volume != nil {
			it.VolumeNano = *o.Volume
		}
		out = append(out, it)
	}
	return out, nil
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func (m *MRKT) doJSON(ctx context.Context, method, path string, body any) ([]byte, error) {
	if m.auth == nil {
		return nil, fmt.Errorf("market: auth provider is required")
	}
	token, err := m.auth.Token(ctx)
	if err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("market: marshal: %w", err)
		}
		reqBody = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, m.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("market: new request: %w", err)
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Cookie", "access_token="+token)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://cdn.tgmrkt.io")
	req.Header.Set("Referer", "https://cdn.tgmrkt.io/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; order-bot/1.0)")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

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
	return raw, nil
}
