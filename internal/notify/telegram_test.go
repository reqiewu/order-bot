package notify

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/reqiewu/order-bot/internal/catalog"
	"github.com/reqiewu/order-bot/internal/engine"
	"github.com/reqiewu/order-bot/internal/fx"
	"github.com/reqiewu/order-bot/internal/marketport"
)

func TestBuyKeyboardGetgemsSell(t *testing.T) {
	t.Parallel()
	s := engine.Signal{
		BuyMarket:  marketport.MarketMRKT,
		SellMarket: marketport.MarketGetgems,
		Lot:        catalog.Lot{URL: "https://cdn.tgmrkt.io/nft/1"},
		SellURL:    "https://getgems.io/nft/EQ-lot",
	}
	m := buyKeyboard(s)
	if m == nil {
		t.Fatal("nil keyboard")
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Inline [][]struct {
			Text string `json:"text"`
			URL  string `json:"url"`
		} `json:"inline_keyboard"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Inline) != 1 || len(parsed.Inline[0]) != 2 {
		t.Fatalf("buttons=%s", raw)
	}
	buy, sell := parsed.Inline[0][0], parsed.Inline[0][1]
	if buy.Text != "Купить MRKT" || buy.URL != s.Lot.URL {
		t.Fatalf("buy %+v", buy)
	}
	if sell.Text != "Купить Getgems" || sell.URL != s.SellURL {
		t.Fatalf("sell %+v", sell)
	}
}

func TestBuyKeyboardGetgemsBuy(t *testing.T) {
	t.Parallel()
	s := engine.Signal{
		BuyMarket:  marketport.MarketGetgems,
		SellMarket: marketport.MarketPortals,
		Lot:        catalog.Lot{URL: "https://getgems.io/nft/EQ-cheap"},
		SellURL:    "https://portals.example/nft/1",
	}
	m := buyKeyboard(s)
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Inline [][]struct {
			Text string `json:"text"`
			URL  string `json:"url"`
		} `json:"inline_keyboard"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Inline) != 1 || len(parsed.Inline[0]) != 2 {
		t.Fatalf("buttons=%s", raw)
	}
	buy, sell := parsed.Inline[0][0], parsed.Inline[0][1]
	if buy.Text != "Купить Getgems" || buy.URL != s.Lot.URL {
		t.Fatalf("buy %+v", buy)
	}
	if sell.Text != "Купить Portals" || sell.URL != s.SellURL {
		t.Fatalf("sell %+v", sell)
	}
}

func TestBuyKeyboardTonnel(t *testing.T) {
	t.Parallel()
	s := engine.Signal{
		BuyMarket:  marketport.MarketTonnel,
		SellMarket: marketport.MarketGetgems,
		Lot:        catalog.Lot{URL: "https://marketplace.tonnel.network/nft/4242"},
		SellURL:    "https://getgems.io/nft/EQ-lot",
	}
	m := buyKeyboard(s)
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Inline [][]struct {
			Text string `json:"text"`
			URL  string `json:"url"`
		} `json:"inline_keyboard"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Inline) != 1 || len(parsed.Inline[0]) != 2 {
		t.Fatalf("buttons=%s", raw)
	}
	buy, sell := parsed.Inline[0][0], parsed.Inline[0][1]
	if buy.Text != "Купить Tonnel" || buy.URL != s.Lot.URL {
		t.Fatalf("buy %+v", buy)
	}
	if sell.Text != "Купить Getgems" || sell.URL != s.SellURL {
		t.Fatalf("sell %+v", sell)
	}
}

func TestFormatPaperTGEscapesHTML(t *testing.T) {
	t.Parallel()
	n := 1
	s := engine.Signal{
		BuyMarket:  "<script>x</script>",
		SellMarket: "MRKT&Co",
		Lot: catalog.Lot{
			ModelBG: catalog.ModelBG{Collection: "Lunar Snake"},
			Number:  &n,
		},
	}
	text := formatPaperTG(s, fx.Quote{})
	if strings.Contains(text, "<script>") {
		t.Fatalf("unescaped: %s", text)
	}
	if !strings.Contains(text, "&lt;script&gt;") {
		t.Fatalf("missing escape: %s", text)
	}
	if !strings.Contains(text, "MRKT&amp;Co") {
		t.Fatalf("amp not escaped: %s", text)
	}
}
