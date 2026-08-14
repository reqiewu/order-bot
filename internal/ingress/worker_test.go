package ingress

import (
	"context"
	"testing"

	"github.com/reqiewu/order-bot/internal/applog"
	"github.com/reqiewu/order-bot/internal/catalog"
	"github.com/reqiewu/order-bot/internal/marketport"
)

type offReader struct{}

func (offReader) List(context.Context, marketport.WatchItem) ([]marketport.Listing, error) {
	panic("List must not run without token")
}

func (offReader) RecentSales(context.Context, marketport.Listing, int) ([]marketport.Sale, error) {
	panic("RecentSales must not run without token")
}

func (offReader) Enabled() bool { return false }

func TestTickSkipsDisabledMarket(t *testing.T) {
	w := &Worker{
		Reader: Reader{Name: "getgems", Client: offReader{}},
		Slots: func() []catalog.WatchSlot {
			return []catalog.WatchSlot{{ID: "1", Collection: "Loot Bag"}}
		},
	}
	w.tick(context.Background(), applog.Nop())
}
