package engine_test

import (
	"testing"

	"github.com/reqiewu/order-bot/internal/engine"
	"github.com/reqiewu/order-bot/internal/money"
)

func TestDedupBadToGood(t *testing.T) {
	d := engine.NewMemoryDeduper()
	buy := money.FromTONFloat(8)
	if d.ShouldAlert("mrkt", "1", buy, false) {
		t.Fatal("bad should not alert")
	}
	d.Remember("mrkt", "1", buy, false)
	if !d.ShouldAlert("mrkt", "1", buy, true) {
		t.Fatal("bad→good should alert")
	}
	d.Remember("mrkt", "1", buy, true)
	if d.ShouldAlert("mrkt", "1", buy, true) {
		t.Fatal("already good should skip")
	}
}

func TestDedupNewGood(t *testing.T) {
	d := engine.NewMemoryDeduper()
	if !d.ShouldAlert("portals", "x", 1, true) {
		t.Fatal("new good should alert")
	}
}
