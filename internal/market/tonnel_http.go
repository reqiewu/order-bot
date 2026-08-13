package market

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
)

const (
	tonnelOrigin  = "https://marketplace.tonnel.network"
	tonnelReferer = "https://marketplace.tonnel.network/"
	// Должен совпадать с профилем Chrome_133 в tonnel_chrome.go.
	tonnelChromeUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36"
)

type tonnelDoer interface {
	Do(ctx context.Context, method, url string, body []byte) (status int, resp []byte, err error)
}

func newTonnelDoer(c *http.Client) tonnelDoer {
	if c != nil {
		return stdHTTPDoer{c: c}
	}
	return &lazyChromeDoer{}
}

type stdHTTPDoer struct {
	c *http.Client
}

func (d stdHTTPDoer) Do(ctx context.Context, method, url string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", tonnelOrigin)
	req.Header.Set("Referer", tonnelReferer)
	req.Header.Set("User-Agent", tonnelChromeUA)
	res, err := d.c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read: %w", err)
	}
	return res.StatusCode, raw, nil
}

type lazyChromeDoer struct {
	once sync.Once
	d    tonnelDoer
	err  error
}

func (l *lazyChromeDoer) Do(ctx context.Context, method, url string, body []byte) (int, []byte, error) {
	l.once.Do(func() {
		l.d, l.err = newChromeTLSDoer()
	})
	if l.err != nil {
		return 0, nil, l.err
	}
	return l.d.Do(ctx, method, url, body)
}
