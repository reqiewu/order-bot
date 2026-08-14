// Package marketport — порт чтения маркетов (адаптировано из gift-bot/dealfinder).
package marketport

import (
	"context"
	"time"
)

const (
	MarketMRKT     = "mrkt"
	MarketPortals  = "portals"
	MarketGetgems  = "getgems"
	MarketTonnel   = "tonnel"
	MarketTelegram = "telegram" // in-app Gift Marketplace (MTProto)

	// DefaultSaleLimit — сколько sales тянуть с API за раз (нужно ≥5 после чистки).
	DefaultSaleLimit = 50

	// DefaultListLimit — сколько самых дешёвых asks брать за List (не полный стакан).
	DefaultListLimit = 100
)

// TakeCheapest возвращает до limit лотов с минимальной Price (стабильный порядок).
func TakeCheapest(in []Listing, limit int) []Listing {
	if limit <= 0 || len(in) <= limit {
		if limit <= 0 {
			return in
		}
		out := make([]Listing, len(in))
		copy(out, in)
		return out
	}
	out := make([]Listing, len(in))
	copy(out, in)
	// insertion-select: для N≤пары сотен достаточно
	for i := 0; i < limit; i++ {
		best := i
		for j := i + 1; j < len(out); j++ {
			if out[j].Price < out[best].Price {
				best = j
			}
		}
		out[i], out[best] = out[best], out[i]
	}
	return out[:limit]
}

// WatchItem — слот мониторинга (коллекция обязательна; model/backdrop опциональны).
type WatchItem struct {
	Collection string
	Model      string
	Backdrop   string
}

// Listing — активный лот (цена в TON float на границе API; ядро конвертирует в nanoTON).
type Listing struct {
	ID         string
	Collection string
	Model      string
	Price      float64
	Number     *int
	Backdrop   string
	Symbol     string
	URL        string
	Market     string // mrkt | portals | getgems | tonnel | telegram — заполняет ingress
}

// Sale — недавняя продажа.
type Sale struct {
	Price      float64
	Number     *int
	At         time.Time
	Collection string
	Model      string
	Backdrop   string
}

// MarketReader — List + RecentSales.
type MarketReader interface {
	List(ctx context.Context, watch WatchItem) ([]Listing, error)
	RecentSales(ctx context.Context, like Listing, limit int) ([]Sale, error)
}
