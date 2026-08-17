package market

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ExchangeGetgemsAuth меняет Telegram Mini App initData на ключ Getgems, если API это умеет.
func ExchangeGetgemsAuth(ctx context.Context, initData string) (string, error) {
	initData = NormalizePortalsTMA(initData)
	if initData == "" {
		return "", ErrEmptyToken
	}
	urls := []string{
		"https://api.getgems.io/auth",
		"https://api.getgems.io/public-api/v1/auth",
	}
	bodies := []map[string]string{
		{"initData": initData},
		{"data": initData},
	}
	var last error
	for _, u := range urls {
		for _, body := range bodies {
			tok, err := postGetgemsAuth(ctx, u, body)
			if err == nil && tok != "" {
				return tok, nil
			}
			last = err
		}
	}
	if last == nil {
		last = fmt.Errorf("market/getgems: auth response has no token")
	}
	return "", last
}

func postGetgemsAuth(ctx context.Context, authURL string, body map[string]string) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; order-bot/1.0)")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("market/getgems: auth HTTP %d: %s", res.StatusCode, truncate(raw, 180))
	}
	var parsed struct {
		Token       string `json:"token"`
		APIKey      string `json:"apiKey"`
		APIKeyAlt   string `json:"api_key"`
		AccessToken string `json:"access_token"`
		Response    struct {
			Token  string `json:"token"`
			APIKey string `json:"apiKey"`
		} `json:"response"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	for _, tok := range []string{parsed.Token, parsed.APIKey, parsed.APIKeyAlt, parsed.AccessToken, parsed.Response.Token, parsed.Response.APIKey} {
		tok = NormalizeGetgemsAPIKey(tok)
		if tok != "" {
			return tok, nil
		}
	}
	return "", fmt.Errorf("market/getgems: auth response has no token")
}
