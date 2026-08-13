package spread_test

import (
	"testing"

	"github.com/reqiewu/order-bot/internal/money"
	"github.com/reqiewu/order-bot/internal/spread"
)

func ton(v float64) money.NanoTON { return money.FromTONFloat(v) }

func TestNetCrossExample(t *testing.T) {
	f := spread.DefaultFees()
	// Buy 8, Sell 9.5 → 9.5*0.95 - 8 - 0.6 = 0.425 (0.3+0.25+0.05)
	net, ok := spread.NetCross(ton(8), ton(9.5), f)
	if !ok {
		t.Fatal("expected ok")
	}
	want := ton(0.425)
	// float→nano may be off by 1; allow small delta
	if diff(net, want) > 2 {
		t.Fatalf("net=%d want≈%d", net, want)
	}
	if !spread.TriggerOK(ton(8), net, f) {
		t.Fatal("expected trigger")
	}
}

func TestNetCrossBelowThreshold(t *testing.T) {
	f := spread.DefaultFees()
	net, ok := spread.NetCross(ton(8), ton(8.5), f)
	if !ok {
		// 8.5*0.95=8.075 - 8 - 0.6 < 0
		return
	}
	if spread.TriggerOK(ton(8), net, f) {
		t.Fatalf("should not trigger, net=%d", net)
	}
}

func TestCleanSalesOutliers(t *testing.T) {
	raw := []money.NanoTON{
		ton(8.0), ton(8.1), ton(8.2), ton(8.3), ton(8.4), ton(25.0), ton(0.5),
	}
	clean := spread.CleanSales(raw, ton(8.2))
	if len(clean) != 5 {
		t.Fatalf("clean=%d want 5: %v", len(clean), clean)
	}
	med, n, ok := spread.MedianClean(raw, ton(8.2), 5)
	if !ok || n != 5 {
		t.Fatalf("ok=%v n=%d", ok, n)
	}
	if diff(med, ton(8.2)) > ton(0.05) {
		t.Fatalf("median=%d", med)
	}
}

func TestFullTriggerRequiresBoth(t *testing.T) {
	f := spread.DefaultFees()
	buy := ton(8)
	ask := spread.EvalFromAsk(buy, ton(9.5), f)
	sales := spread.EvalFromSales(buy, []money.NanoTON{
		ton(9.4), ton(9.5), ton(9.6), ton(9.55), ton(9.45),
	}, ton(9.5), f)
	if !spread.FullTrigger(ask, sales) {
		t.Fatalf("ask=%+v sales=%+v", ask, sales)
	}
	weak := spread.EvalFromSales(buy, []money.NanoTON{ton(9.5), ton(9.6)}, ton(9.5), f)
	if spread.FullTrigger(ask, weak) {
		t.Fatal("should fail with <5 sales")
	}
}

func diff(a, b money.NanoTON) money.NanoTON {
	if a > b {
		return a - b
	}
	return b - a
}
