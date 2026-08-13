package giftid_test

import (
	"testing"

	"github.com/reqiewu/order-bot/internal/giftid"
)

func TestTelegramGiftID(t *testing.T) {
	t.Parallel()
	if got := giftid.TelegramGiftID("Fine Pen", 15525); got != "FinePen-15525" {
		t.Fatalf("got %q", got)
	}
	if got := giftid.TelegramNFTURL("Fine Pen", 15525); got != "https://t.me/nft/FinePen-15525" {
		t.Fatalf("got %q", got)
	}
}

func TestPortalsCollectionUsesShortName(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Lunar Snake":  "lunarsnake",
		"Ice Cream":    "icecream",
		"Astral Shard": "astralshard",
		"Plush Pepe":   "plushpepe",
		"Durov's Cap":  "durovscap",
		"Durov’s Cap":  "durovscap", // curly apostrophe
		"lunarsnake":   "lunarsnake",
	}
	for in, want := range cases {
		if got := giftid.PortalsCollection(in); got != want {
			t.Fatalf("PortalsCollection(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestMRKTNamesRestoresDisplayFromShortName(t *testing.T) {
	t.Parallel()
	got := giftid.MRKTNames(giftid.Identity{
		Collection: "lunarsnake",
		Model:      "Albino",
		Backdrop:   "Black",
	})
	if got.Collection != "Lunar Snake" {
		t.Fatalf("Collection=%q, want Lunar Snake", got.Collection)
	}
	if got.Model != "Albino" || got.Backdrop != "Black" {
		t.Fatalf("attrs = %+v", got)
	}
}

func TestPortalsNamesFoldsCollectionKeepsModelBackdrop(t *testing.T) {
	t.Parallel()
	got := giftid.PortalsNames(giftid.Identity{
		Collection: "Lunar Snake",
		Model:      "Albino",
		Backdrop:   "Sky Blue",
	})
	if got.Collection != "lunarsnake" {
		t.Fatalf("Collection=%q", got.Collection)
	}
	if got.Model != "Albino" || got.Backdrop != "Sky Blue" {
		t.Fatalf("attrs = %+v", got)
	}
}

func TestSameCollection(t *testing.T) {
	t.Parallel()
	if !giftid.SameCollection("Lunar Snake", "lunarsnake") {
		t.Fatal("expected same")
	}
	if giftid.SameCollection("Lunar Snake", "Ice Cream") {
		t.Fatal("expected different")
	}
}

func TestMRKTNamesPassthroughDisplay(t *testing.T) {
	t.Parallel()
	got := giftid.MRKTNames(giftid.Identity{Collection: "Lunar Snake", Model: "Albino"})
	if got.Collection != "Lunar Snake" {
		t.Fatalf("got %q", got.Collection)
	}
}
