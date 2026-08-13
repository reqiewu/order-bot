package market

import (
	"bytes"
	"context"
	"fmt"
	"io"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type chromeTLSDoer struct {
	c tls_client.HttpClient
}

func newChromeTLSDoer() (tonnelDoer, error) {
	c, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(),
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_133),
		tls_client.WithRandomTLSExtensionOrder(),
		tls_client.WithCookieJar(tls_client.NewCookieJar()),
	)
	if err != nil {
		return nil, fmt.Errorf("market/tonnel: chrome tls client: %w", err)
	}
	return &chromeTLSDoer{c: c}, nil
}

func (d *chromeTLSDoer) Do(ctx context.Context, method, url string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header = http.Header{
		"content-type":       {"application/json"},
		"accept":             {"application/json"},
		"accept-language":    {"en-US,en;q=0.9"},
		"origin":             {tonnelOrigin},
		"referer":            {tonnelReferer},
		"sec-ch-ua":          {`"Not(A:Brand";v="8", "Chromium";v="133", "Google Chrome";v="133"`},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"macOS"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {"same-site"},
		"user-agent":         {tonnelChromeUA},
		http.HeaderOrderKey: {
			"content-type",
			"accept",
			"accept-language",
			"origin",
			"referer",
			"sec-ch-ua",
			"sec-ch-ua-mobile",
			"sec-ch-ua-platform",
			"sec-fetch-dest",
			"sec-fetch-mode",
			"sec-fetch-site",
			"user-agent",
		},
		http.PHeaderOrderKey: {
			":method",
			":authority",
			":scheme",
			":path",
		},
	}
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
