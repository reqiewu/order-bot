package market

import (
	"testing"

	"github.com/reqiewu/order-bot/internal/marketport"
)

func TestNormalizeTelegramUniqueTON(t *testing.T) {
	lot, ok := normalizeTelegramUnique(tgUniqueView{
		ID: 1, Title: "Loot Bag", Slug: "LootBag-1", Num: 1,
		Model: "Academic", Backdrop: "Black",
		TonNano: 2_500_000_000,
		Collection: "Loot Bag",
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if lot.Market != marketport.MarketTelegram {
		t.Fatalf("market %s", lot.Market)
	}
	if lot.Price != 2.5 {
		t.Fatalf("price %v", lot.Price)
	}
	if lot.URL != "https://t.me/nft/LootBag-1" {
		t.Fatalf("url %s", lot.URL)
	}
	if lot.Model != "Academic" || lot.Backdrop != "Black" {
		t.Fatalf("attrs %+v", lot)
	}
}

func TestNormalizeTelegramUniqueSkipStarsOnly(t *testing.T) {
	_, ok := normalizeTelegramUnique(tgUniqueView{
		ID: 1, Title: "Loot Bag", Collection: "Loot Bag", TonNano: 0,
	})
	if ok {
		t.Fatal("stars-only must skip")
	}
}

func TestTonPriceFromAmounts(t *testing.T) {
	n := tonPriceFromAmounts([]tgAmountView{
		{IsTON: false, Amount: 100},
		{IsTON: true, Amount: 1_000_000_000},
	})
	if n != 1_000_000_000 {
		t.Fatalf("got %d", n)
	}
}

func TestMatchTelegramWatch(t *testing.T) {
	lot := marketport.Listing{Collection: "Loot Bag", Model: "Academic", Backdrop: "Black"}
	if !matchTelegramWatch(lot, marketport.WatchItem{Collection: "Loot Bag"}) {
		t.Fatal("collection only")
	}
	if matchTelegramWatch(lot, marketport.WatchItem{Collection: "Loot Bag", Model: "Other"}) {
		t.Fatal("model mismatch")
	}
	if !matchTelegramWatch(lot, marketport.WatchItem{Collection: "lootbag", Model: "academic", Backdrop: "black"}) {
		t.Fatal("fold match")
	}
}
