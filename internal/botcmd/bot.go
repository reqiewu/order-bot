package botcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/reqiewu/order-bot/internal/applog"
)

// BotDescription — экран до «Начать» (setMyDescription).
func BotDescription() string {
	return "order-bot — paper-снайп Telegram Gifts (Portals ↔ MRKT).\n" +
		"Нажми «Начать» → открой Mini App и настрой слоты."
}

// BotShortDescription — профиль бота (≤120).
func BotShortDescription() string {
	return "Paper-снайп Gifts: Portals ↔ MRKT."
}

func WelcomeHTML() string {
	return "👋 <b>order-bot</b> — paper-снайп кросс-маркета.\n\n" +
		"Бот следит за листингами <b>Portals</b> и <b>MRKT</b> " +
		"и шлёт алерт, если после комиссий есть спред.\n\n" +
		"⚙️ Настройки и whitelist — в <b>Mini App</b> " +
		"(кнопка меню или /app).\n\n" +
		"Режим: <b>paper</b> (без автопокупки)."
}

// Bot — long-poll + описание + /start|/app.
type Bot struct {
	token      string
	apiBase    string
	http       *http.Client
	operatorID int64
	webAppURL  string
	log        *applog.Logger
}

type Config struct {
	Token      string
	OperatorID int64
	WebAppURL  string // HTTPS Mini App; пусто → без web_app кнопки
	Log        *applog.Logger
	HTTP       *http.Client
	APIBase    string
}

func New(cfg Config) *Bot {
	base := strings.TrimRight(cfg.APIBase, "/")
	if base == "" {
		base = "https://api.telegram.org"
	}
	client := cfg.HTTP
	if client == nil {
		client = &http.Client{Timeout: 35 * time.Second}
	}
	log := cfg.Log
	if log == nil {
		log = applog.Nop()
	}
	return &Bot{
		token:      cfg.Token,
		apiBase:    base,
		http:       client,
		operatorID: cfg.OperatorID,
		webAppURL:  strings.TrimSpace(cfg.WebAppURL),
		log:        log,
	}
}

type update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Text string `json:"text"`
		From *struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}

// Run — setMy* + getUpdates, пока жив ctx.
func (b *Bot) Run(ctx context.Context) error {
	if b.token == "" {
		return fmt.Errorf("botcmd: empty token")
	}
	if err := b.setMyCommands(ctx); err != nil {
		b.log.Warn("setMyCommands failed", "err", err)
	}
	if err := b.setMyDescription(ctx); err != nil {
		b.log.Warn("setMyDescription failed", "err", err)
	}
	if err := b.setMyShortDescription(ctx); err != nil {
		b.log.Warn("setMyShortDescription failed", "err", err)
	}

	offset := int64(0)
	b.log.Info("command bot listening")
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		updates, err := b.getUpdates(ctx, offset)
		if err != nil {
			b.log.Warn("getUpdates failed", "err", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		}
		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			b.handle(ctx, u)
		}
	}
}

func (b *Bot) handle(ctx context.Context, u update) {
	if u.Message == nil || u.Message.From == nil {
		return
	}
	chatID := u.Message.Chat.ID
	userID := u.Message.From.ID
	text := strings.TrimSpace(u.Message.Text)
	cmd, _, _ := strings.Cut(text, " ")
	cmd = strings.ToLower(strings.Split(cmd, "@")[0])

	if b.operatorID > 0 && userID != b.operatorID {
		if cmd == "/start" {
			_ = b.sendHTML(ctx, chatID, "⛔ Личный бот. Доступ только у оператора.")
		}
		return
	}

	switch cmd {
	case "/start", "/help":
		_ = b.sendWelcome(ctx, chatID)
	case "/app":
		_ = b.sendWelcome(ctx, chatID)
	}
}

func (b *Bot) sendWelcome(ctx context.Context, chatID int64) error {
	markup := ""
	if b.webAppURL != "" {
		kb, _ := json.Marshal(map[string]any{
			"inline_keyboard": [][]map[string]any{{
				{"text": "Открыть Mini App", "web_app": map[string]string{"url": b.webAppURL}},
			}},
		})
		markup = string(kb)
	}
	return b.sendHTML(ctx, chatID, WelcomeHTML(), markup)
}

func (b *Bot) setMyCommands(ctx context.Context) error {
	cmds, _ := json.Marshal([]map[string]string{
		{"command": "start", "description": "Приветствие и Mini App"},
		{"command": "app", "description": "Открыть Mini App"},
		{"command": "help", "description": "Справка"},
	})
	form := url.Values{}
	form.Set("commands", string(cmds))
	return b.postForm(ctx, "setMyCommands", form)
}

func (b *Bot) setMyDescription(ctx context.Context) error {
	form := url.Values{}
	form.Set("description", BotDescription())
	return b.postForm(ctx, "setMyDescription", form)
}

func (b *Bot) setMyShortDescription(ctx context.Context) error {
	form := url.Values{}
	form.Set("short_description", BotShortDescription())
	return b.postForm(ctx, "setMyShortDescription", form)
}

func (b *Bot) sendHTML(ctx context.Context, chatID int64, html string, replyMarkup ...string) error {
	form := url.Values{}
	form.Set("chat_id", strconv.FormatInt(chatID, 10))
	form.Set("text", html)
	form.Set("parse_mode", "HTML")
	form.Set("disable_web_page_preview", "true")
	if len(replyMarkup) > 0 && replyMarkup[0] != "" {
		form.Set("reply_markup", replyMarkup[0])
	}
	return b.postForm(ctx, "sendMessage", form)
}

func (b *Bot) postForm(ctx context.Context, method string, form url.Values) error {
	endpoint := fmt.Sprintf("%s/bot%s/%s", b.apiBase, b.token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := b.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s status %d: %s", method, res.StatusCode, truncate(string(body), 200))
	}
	var parsed struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return err
	}
	if !parsed.OK {
		return fmt.Errorf("%s ok=false: %s", method, truncate(string(body), 200))
	}
	return nil
}

func (b *Bot) getUpdates(ctx context.Context, offset int64) ([]update, error) {
	q := url.Values{}
	q.Set("timeout", "25")
	q.Set("offset", strconv.FormatInt(offset, 10))
	q.Set("allowed_updates", `["message"]`)
	endpoint := fmt.Sprintf("%s/bot%s/getUpdates?%s", b.apiBase, b.token, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	// long poll
	client := b.http
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("getUpdates status %d: %s", res.StatusCode, truncate(string(body), 200))
	}
	var parsed struct {
		OK     bool     `json:"ok"`
		Result []update `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if !parsed.OK {
		return nil, fmt.Errorf("getUpdates ok=false")
	}
	return parsed.Result, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
