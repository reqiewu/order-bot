// Package marketport — порт чтения маркетов (адаптировано из gift-bot/dealfinder).
package marketport

import (
	"context"
	"time"
)

const (
	MarketMRKT    = "mrkt"
	MarketPortals = "portals"
	MarketGetgems = "getgems"

	// DefaultSaleLimit — сколько sales тянуть с API за раз (нужно ≥5 после чистки).
	DefaultSaleLimit = 50
)

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
	Market     string // mrkt | portals | getgems — заполняет ingress
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
