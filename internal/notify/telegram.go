package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/reqiewu/order-bot/internal/applog"
	"github.com/reqiewu/order-bot/internal/engine"
	"github.com/reqiewu/order-bot/internal/fx"
	"github.com/reqiewu/order-bot/internal/giftid"
	"github.com/reqiewu/order-bot/internal/marketport"
	"github.com/reqiewu/order-bot/internal/money"
	"github.com/reqiewu/order-bot/internal/spread"
)

// Telegram — простой sendMessage (paper alerts).
type Telegram struct {
	Token  string
	ChatID int64
	HTTP   *http.Client
	Rates  fx.RateSource // справочный TON/USD (CMC); опционально
}

func (t *Telegram) PaperSignal(s engine.Signal) error {
	if t.Token == "" || t.ChatID == 0 {
		return fmt.Errorf("notify: telegram not configured")
	}
	var q fx.Quote
	if t.Rates != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		got, err := t.Rates.Rate(ctx)
		cancel()
		if err == nil {
			q = got
		}
	}
	text := formatPaperTG(s, q)
	body := map[string]any{
		"chat_id":                  t.ChatID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}
	if markup := buyKeyboard(s); markup != nil {
		body["reply_markup"] = markup
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

func formatPaperTG(s engine.Signal, q fx.Quote) string {
	fees := feesTotal(s.BestAsk)
	var b strings.Builder
	fmt.Fprintf(&b, "<b>PAPER BUY</b> %s @ %s%s\n", s.BuyMarket, formatTON(s.Lot.Price), usdtSuffix(s.Lot.Price, q))
	fmt.Fprintf(&b, "<b>SELL</b> %s @ %s (ask) / undercut %s\n", s.SellMarket, formatTON(s.BestAsk), formatTON(s.Undercut))
	fmt.Fprintf(&b, "comps median %s\n", formatTON(s.SalesMedian))
	fmt.Fprintf(&b, "net ask ≈ %s%s | net sales ≈ %s%s\n",
		formatTON(s.NetAsk), usdtSuffix(s.NetAsk, q),
		formatTON(s.NetSales), usdtSuffix(s.NetSales, q),
	)
	fmt.Fprintf(&b, "fees: %s%s\n", formatTON(fees), usdtSuffix(fees, q))
	if s.CollFloor > 0 {
		fmt.Fprintf(&b, "collection floor: %s\n", formatTON(s.CollFloor))
	}
	if nft := nftURL(s); nft != "" {
		fmt.Fprintf(&b, "%s", nft)
	}
	return b.String()
}

func usdtSuffix(n money.NanoTON, q fx.Quote) string {
	if q.USDPerTON <= 0 || n == 0 {
		return ""
	}
	usd := float64(n) / float64(money.TON) * q.USDPerTON
	return fmt.Sprintf(" (~%.2f USDT)", usd)
}

func feesTotal(bestAsk money.NanoTON) money.NanoTON {
	f := spread.DefaultFees()
	return money.MulBPS(bestAsk, f.SellFeeBPS) + f.Withdraw + f.Transfer + f.Gas
}

func nftURL(s engine.Signal) string {
	if s.Lot.Number != nil {
		if u := giftid.TelegramNFTURL(s.Lot.ModelBG.Collection, *s.Lot.Number); u != "" {
			return u
		}
	}
	return ""
}

func buyKeyboard(s engine.Signal) map[string]any {
	type btn struct {
		Text string `json:"text"`
		URL  string `json:"url"`
	}
	var row []btn
	buyURL := strings.TrimSpace(s.Lot.URL)
	sellURL := strings.TrimSpace(s.SellURL)
	if label, url := marketBtn(s.BuyMarket, buyURL); url != "" {
		row = append(row, btn{Text: label, URL: url})
	}
	if label, url := marketBtn(s.SellMarket, sellURL); url != "" {
		row = append(row, btn{Text: label, URL: url})
	}
	if len(row) == 0 {
		return nil
	}
	return map[string]any{
		"inline_keyboard": [][]btn{row},
	}
}

func marketBtn(market, url string) (string, string) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", ""
	}
	switch market {
	case marketport.MarketPortals:
		return "Купить Portals", url
	case marketport.MarketMRKT:
		return "Купить MRKT", url
	case marketport.MarketGetgems:
		return "Купить Getgems", url
	default:
		return "", ""
	}
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

// LogAlerter — paper alert в дерево логов.
type LogAlerter struct {
	Log   *applog.Logger
	Rates fx.RateSource
}

func (l LogAlerter) PaperSignal(s engine.Signal) error {
	log := l.Log
	if log == nil {
		log = applog.Nop()
	}
	kvs := []applog.KV{
		{K: "buy", V: fmt.Sprintf("%s @ %s", s.BuyMarket, formatTON(s.Lot.Price))},
		{K: "sell", V: fmt.Sprintf("%s ask %s / undercut %s", s.SellMarket, formatTON(s.BestAsk), formatTON(s.Undercut))},
		{K: "comps", V: formatTON(s.SalesMedian)},
		{K: "net", V: fmt.Sprintf("ask %s | sales %s", formatTON(s.NetAsk), formatTON(s.NetSales))},
		{K: "gift", V: s.Lot.ModelBG.String()},
	}
	if s.CollFloor > 0 {
		kvs = append(kvs, applog.KV{K: "floor", V: formatTON(s.CollFloor)})
	}
	if s.Lot.Number != nil {
		kvs = append(kvs, applog.KV{K: "number", V: fmt.Sprintf("#%d", *s.Lot.Number)})
	}
	if s.Lot.URL != "" {
		kvs = append(kvs, applog.KV{K: "url", V: s.Lot.URL})
	}
	if l.Rates != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		q, err := l.Rates.Rate(ctx)
		cancel()
		if err == nil && q.USDPerTON > 0 {
			kvs = append(kvs, applog.KV{
				K: "ton_usdt",
				V: fmt.Sprintf("%.4g (%s)", q.USDPerTON, q.Source),
			})
		}
	}
	log.InfoTree("paper alert", kvs...)
	return nil
}

// Multi шлёт сигнал во все alerter'ы (лог + Telegram).
type Multi struct {
	Alerts []engine.Alerter
}

func (m Multi) PaperSignal(s engine.Signal) error {
	var first error
	for _, a := range m.Alerts {
		if a == nil {
			continue
		}
		if err := a.PaperSignal(s); err != nil && first == nil {
			first = err
		}
	}
	return first
}
