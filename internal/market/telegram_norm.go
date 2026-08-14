package market

import (
	"strconv"
	"strings"

	"github.com/reqiewu/order-bot/internal/giftid"
	"github.com/reqiewu/order-bot/internal/marketport"
)

const nanoTONInt = 1_000_000_000

// tgUniqueView — нормализуемый снимок starGiftUnique (без зависимости от gotd в тестах).
type tgUniqueView struct {
	ID         int64
	Title      string
	Slug       string
	Num        int
	Model      string
	Backdrop   string
	Symbol     string
	TonNano    int64 // 0 = нет TON-цены (Stars-only → skip)
	Collection string // override; иначе Title
}

func tonPriceFromAmounts(amounts []tgAmountView) int64 {
	for _, a := range amounts {
		if a.IsTON && a.Amount > 0 {
			return a.Amount
		}
	}
	return 0
}

type tgAmountView struct {
	IsTON  bool
	Amount int64
}

func normalizeTelegramUnique(v tgUniqueView) (marketport.Listing, bool) {
	if v.TonNano <= 0 {
		return marketport.Listing{}, false
	}
	coll := strings.TrimSpace(v.Collection)
	if coll == "" {
		coll = strings.TrimSpace(v.Title)
	}
	if coll == "" {
		return marketport.Listing{}, false
	}
	price := float64(v.TonNano) / nanoTONInt
	id := strings.TrimSpace(v.Slug)
	if id == "" {
		id = strconv.FormatInt(v.ID, 10)
	}
	num := v.Num
	var numPtr *int
	if num > 0 {
		n := num
		numPtr = &n
	}
	url := ""
	if slug := strings.TrimSpace(v.Slug); slug != "" {
		url = "https://t.me/nft/" + slug
	}
	return marketport.Listing{
		ID:         id,
		Collection: coll,
		Model:      strings.TrimSpace(v.Model),
		Backdrop:   strings.TrimSpace(v.Backdrop),
		Symbol:     strings.TrimSpace(v.Symbol),
		Price:      price,
		Number:     numPtr,
		URL:        url,
		Market:     marketport.MarketTelegram,
	}, true
}

func matchTelegramWatch(lot marketport.Listing, watch marketport.WatchItem) bool {
	if !giftid.SameCollection(lot.Collection, watch.Collection) {
		return false
	}
	if want := giftid.Fold(watch.Model); want != "" && giftid.Fold(lot.Model) != want {
		return false
	}
	if want := giftid.Fold(watch.Backdrop); want != "" && giftid.Fold(lot.Backdrop) != want {
		return false
	}
	return true
}
