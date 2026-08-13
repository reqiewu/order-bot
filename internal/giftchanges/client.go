package giftchanges

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const giftChangesBase = "https://api.changes.tg"

// GiftChanges — read-only client for api.changes.tg (catalog + assets metadata).
type GiftChanges struct {
	HTTP *http.Client
}

// NewGiftChanges returns a client with sane defaults.
func NewGiftChanges() *GiftChanges {
	return &GiftChanges{HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// GiftSummary — gift with models/backdrops from GET /gift/:gift.
type GiftSummary struct {
	Name           string         `json:"name"`
	ID             string         `json:"id"`
	CustomEmojiID  string         `json:"customEmojiId"`
	Models         []NamedRarity  `json:"models"`
	Backdrops      []NamedRarity  `json:"backdrops"`
	Symbols        []NamedRarity  `json:"symbols"`
}

type NamedRarity struct {
	Name   string  `json:"name"`
	Rarity float64 `json:"rarity"`
}

// ListGifts returns upgradable gift names (GET /gifts).
func (c *GiftChanges) ListGifts(ctx context.Context) ([]string, error) {
	var names []string
	if err := c.getJSON(ctx, "/gifts", &names); err != nil {
		return nil, err
	}
	return names, nil
}

// Gift returns detailed gift info (GET /gift/:gift).
func (c *GiftChanges) Gift(ctx context.Context, gift string) (GiftSummary, error) {
	var raw struct {
		Gift struct {
			Name          string `json:"name"`
			ID            string `json:"id"`
			CustomEmojiID string `json:"customEmojiId"`
		} `json:"gift"`
		Models    []NamedRarity `json:"models"`
		Backdrops []NamedRarity `json:"backdrops"`
		Symbols   []NamedRarity `json:"symbols"`
	}
	path := "/gift/" + url.PathEscape(normalizeGiftSlug(gift))
	if err := c.getJSON(ctx, path, &raw); err != nil {
		return GiftSummary{}, err
	}
	return GiftSummary{
		Name:          raw.Gift.Name,
		ID:            raw.Gift.ID,
		CustomEmojiID: raw.Gift.CustomEmojiID,
		Models:        raw.Models,
		Backdrops:     raw.Backdrops,
		Symbols:       raw.Symbols,
	}, nil
}

// BackdropInfo — colors from GET /backdrop/:gift/:backdrop/info.
type BackdropInfo struct {
	Name        string `json:"name"`
	CenterColor string `json:"centerColor"`
	EdgeColor   string `json:"edgeColor"`
	PatternColor string `json:"patternColor"`
	TextColor   string `json:"textColor"`
}

// BackdropInfo fetches backdrop colors for preview.
func (c *GiftChanges) BackdropInfo(ctx context.Context, gift, backdrop string) (BackdropInfo, error) {
	var raw struct {
		Name         string `json:"name"`
		Hex          struct {
			CenterColor  string `json:"centerColor"`
			EdgeColor    string `json:"edgeColor"`
			PatternColor string `json:"patternColor"`
			TextColor    string `json:"textColor"`
		} `json:"hex"`
	}
	g := normalizeGiftSlug(gift)
	b := url.PathEscape(strings.TrimSpace(backdrop))
	path := fmt.Sprintf("/backdrop/%s/%s/info", g, b)
	if err := c.getJSON(ctx, path, &raw); err != nil {
		return BackdropInfo{}, err
	}
	return BackdropInfo{
		Name:         raw.Name,
		CenterColor:  raw.Hex.CenterColor,
		EdgeColor:    raw.Hex.EdgeColor,
		PatternColor: raw.Hex.PatternColor,
		TextColor:    raw.Hex.TextColor,
	}, nil
}

// SymbolPNGURL — pattern overlay for preview.
func SymbolPNGURL(gift, symbol string, size int) string {
	if size <= 0 {
		size = 128
	}
	g := normalizeGiftSlug(gift)
	s := strings.ReplaceAll(symbol, " ", "%20")
	return fmt.Sprintf("%s/symbol/%s/%s.png?size=%d", giftChangesBase, g, s, size)
}

// ModelPNGURL — preview image for a model (user-visible assets via GiftChanges CDN rules).
func ModelPNGURL(gift, model string, size int) string {
	if size <= 0 {
		size = 256
	}
	g := normalizeGiftSlug(gift)
	m := strings.ReplaceAll(model, " ", "%20")
	return fmt.Sprintf("%s/model/%s/%s.png?size=%d", giftChangesBase, g, m, size)
}

// OriginalPNGURL — collection icon (GET /original/:gift.png).
func OriginalPNGURL(gift string, size int) string {
	if size <= 0 {
		size = 128
	}
	g := normalizeGiftSlug(gift)
	return fmt.Sprintf("%s/original/%s.png?size=%d", giftChangesBase, g, size)
}

func normalizeGiftSlug(gift string) string {
	gift = strings.TrimSpace(gift)
	if gift == "" {
		return gift
	}
	// API accepts many formats; spaceless is URL-safe.
	return strings.ReplaceAll(gift, " ", "")
}

func (c *GiftChanges) getJSON(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, giftChangesBase+path, nil)
	if err != nil {
		return err
	}
	res, err := c.http().Do(req)
	if err != nil {
		return fmt.Errorf("giftchanges: %s: %w", path, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("giftchanges: %s: status %d: %s", path, res.StatusCode, truncate(body, 200))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("giftchanges: decode %s: %w", path, err)
	}
	return nil
}

func (c *GiftChanges) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
