package market

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ExchangeMRKTAuth меняет Telegram Mini App initData на access_token UUID MRKT.
func ExchangeMRKTAuth(ctx context.Context, initData string) (string, error) {
	return ExchangeMRKTAuthURL(ctx, defaultBaseURL+"/api/v1/auth", initData)
}

// ExchangeMRKTAuthURL — как ExchangeMRKTAuth, но с явным URL (тесты).
func ExchangeMRKTAuthURL(ctx context.Context, authURL, initData string) (string, error) {
	initData = NormalizePortalsTMA(initData)
	if initData == "" {
		return "", ErrEmptyToken
	}
	payload, err := json.Marshal(map[string]string{"data": initData})
	if err != nil {
		return "", fmt.Errorf("market/mrkt: marshal auth: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("market/mrkt: auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://cdn.tgmrkt.io")
	req.Header.Set("Referer", "https://cdn.tgmrkt.io/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; order-bot/1.0)")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("market/mrkt: auth http: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("market/mrkt: auth body: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("market/mrkt: auth HTTP %d: %s", res.StatusCode, truncate(raw, 180))
	}
	var parsed struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("market/mrkt: auth json: %w", err)
	}
	tok := NormalizeMRKTToken(parsed.Token)
	if tok == "" {
		tok = NormalizeMRKTToken(parsed.AccessToken)
	}
	if tok == "" {
		return "", fmt.Errorf("market/mrkt: auth response has no token")
	}
	return tok, nil
}
