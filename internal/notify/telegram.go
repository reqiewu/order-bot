package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/reqiewu/order-bot/internal/engine"
	"github.com/reqiewu/order-bot/internal/money"
)

// Telegram — простой sendMessage (paper alerts).
type Telegram struct {
	Token  string
	ChatID int64
	HTTP   *http.Client
}

func (t *Telegram) PaperSignal(s engine.Signal) error {
	if t.Token == "" || t.ChatID == 0 {
		return fmt.Errorf("notify: telegram not configured")
	}
	text := formatPaper(s)
	body := map[string]any{
		"chat_id":    t.ChatID,
		"text":       text,
		"parse_mode": "HTML",
		"disable_web_page_preview": true,
	}
	raw, _ := json.Marshal(body)
	url := "https://api.telegram.org/bot" + t.Token + "/sendMessage"
	client := t.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("notify: telegram status %s", resp.Status)
	}
	return nil
}

func formatPaper(s engine.Signal) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<b>PAPER BUY</b> %s @ %s\n", s.BuyMarket, formatTON(s.Lot.Price))
	fmt.Fprintf(&b, "<b>SELL</b> %s @ %s (ask) / undercut %s\n", s.SellMarket, formatTON(s.BestAsk), formatTON(s.Undercut))
	fmt.Fprintf(&b, "comps median %s\n", formatTON(s.SalesMedian))
	fmt.Fprintf(&b, "net ask ≈ %s | net sales ≈ %s\n", formatTON(s.NetAsk), formatTON(s.NetSales))
	fmt.Fprintf(&b, "fees: 5%% + 0.3 + 0.25 + 0.05 gas\n")
	if s.CollFloor > 0 {
		fmt.Fprintf(&b, "collection floor: %s\n", formatTON(s.CollFloor))
	}
	fmt.Fprintf(&b, "%s\n", s.Lot.ModelBG.String())
	if s.Lot.Number != nil {
		fmt.Fprintf(&b, "#%d\n", *s.Lot.Number)
	}
	if s.Lot.URL != "" {
		fmt.Fprintf(&b, "%s\n", s.Lot.URL)
	}
	return b.String()
}

func formatTON(n money.NanoTON) string {
	whole := n / money.TON
	frac := n % money.TON
	// 2–4 знака без float ядра
	s := fmt.Sprintf("%d.%09d", whole, frac)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s + " TON"
}

// LogAlerter — для локального прогона без Telegram.
type LogAlerter struct {
	Printf func(format string, args ...any)
}

func (l LogAlerter) PaperSignal(s engine.Signal) error {
	p := l.Printf
	if p == nil {
		p = func(string, ...any) {}
	}
	p("%s", formatPaper(s))
	return nil
}
