package tguser

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// Config — MTProto user session (не bot token).
type Config struct {
	APIID       int
	APIHash     string
	SessionPath string
}

// Client держит долгоживущий gotd-клиент для payments.getResaleStarGifts.
type Client struct {
	cfg Config

	mu    sync.Mutex
	api   *tg.Client
	ready chan struct{}
	runErr error
}

// New создаёт клиент (ещё не подключён).
func New(cfg Config) (*Client, error) {
	if cfg.APIID == 0 || strings.TrimSpace(cfg.APIHash) == "" {
		return nil, fmt.Errorf("tguser: TELEGRAM_API_ID/TELEGRAM_API_HASH required")
	}
	path := strings.TrimSpace(cfg.SessionPath)
	if path == "" {
		path = "data/tg.session"
	}
	cfg.SessionPath = path
	return &Client{
		cfg:   cfg,
		ready: make(chan struct{}),
	}, nil
}

// SessionPath — путь к файлу сессии.
func (c *Client) SessionPath() string { return c.cfg.SessionPath }

// HasSessionFile — есть ли сохранённая сессия.
func (c *Client) HasSessionFile() bool {
	st, err := os.Stat(c.cfg.SessionPath)
	return err == nil && st.Size() > 0
}

// Run блокируется до отмены ctx; поднимает MTProto и держит соединение.
func (c *Client) Run(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(c.cfg.SessionPath), 0o755); err != nil {
		return fmt.Errorf("tguser: session dir: %w", err)
	}
	client := telegram.NewClient(c.cfg.APIID, c.cfg.APIHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: c.cfg.SessionPath},
	})
	err := client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !status.Authorized {
			return fmt.Errorf("tguser: not authorized — run: go run ./cmd/tg-login")
		}
		c.mu.Lock()
		c.api = client.API()
		c.mu.Unlock()
		close(c.ready)
		<-ctx.Done()
		return ctx.Err()
	})
	c.mu.Lock()
	c.runErr = err
	c.mu.Unlock()
	return err
}

// API ждёт готовности (или ctx/ошибки Run).
func (c *Client) API(ctx context.Context) (*tg.Client, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ready:
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.api == nil {
		if c.runErr != nil {
			return nil, c.runErr
		}
		return nil, fmt.Errorf("tguser: api not ready")
	}
	return c.api, nil
}

// WaitReady с таймаутом на старте.
func (c *Client) WaitReady(ctx context.Context, d time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	_, err := c.API(ctx)
	return err
}

// Login интерактивно пишет сессию в SessionPath (для cmd/tg-login).
func Login(ctx context.Context, cfg Config, phone, password string) error {
	if cfg.APIID == 0 || strings.TrimSpace(cfg.APIHash) == "" {
		return fmt.Errorf("tguser: TELEGRAM_API_ID/TELEGRAM_API_HASH required")
	}
	path := strings.TrimSpace(cfg.SessionPath)
	if path == "" {
		path = "data/tg.session"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	phone = strings.TrimSpace(phone)
	if phone == "" {
		fmt.Fprint(os.Stderr, "Phone (+…): ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return err
		}
		phone = strings.TrimSpace(line)
	}
	client := telegram.NewClient(cfg.APIID, cfg.APIHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: path},
	})
	return client.Run(ctx, func(ctx context.Context) error {
		flow := auth.NewFlow(
			auth.Constant(phone, password, auth.CodeAuthenticatorFunc(func(ctx context.Context, _ *tg.AuthSentCode) (string, error) {
				fmt.Fprint(os.Stderr, "Code: ")
				line, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					return "", err
				}
				return strings.TrimSpace(line), nil
			})),
			auth.SendCodeOptions{},
		)
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "OK — session saved to %s\n", path)
		return nil
	})
}
