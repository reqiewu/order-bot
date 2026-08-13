package spread

import (
	"sort"

	"github.com/reqiewu/order-bot/internal/money"
)

// Fees — параметры кросс-сделки (v1 Portals↔MRKT).
type Fees struct {
	SellFeeBPS     uint64
	Withdraw       money.NanoTON
	Transfer       money.NanoTON
	Gas            money.NanoTON
	MinProfit      money.NanoTON
	MinSpreadBPS   uint64
	UndercutStep   money.NanoTON
	MinCleanSales  int
}

// DefaultFees — значения из CONTEXT.md.
func DefaultFees() Fees {
	return Fees{
		SellFeeBPS:    money.SellFeeBPS,
		Withdraw:      money.WithdrawFee,
		Transfer:      money.TransferFee,
		Gas:           money.GasReserve,
		MinProfit:     money.DefaultMinProfit,
		MinSpreadBPS:  money.MinSpreadBPS,
		UndercutStep:  money.DefaultUndercut,
		MinCleanSales: 5,
	}
}

func (f Fees) fixedCross() money.NanoTON {
	return f.Withdraw + f.Transfer + f.Gas
}

// NetCross — Net = Sell×(1−fee) − Buy − fixed.
// Если Sell после fee меньше Buy+fixed, возвращает 0 и ok=false (убыток не кодируем в uint).
func NetCross(buy, sell money.NanoTON, f Fees) (net money.NanoTON, ok bool) {
	afterFee := money.MulBPS(sell, 10_000-f.SellFeeBPS)
	fixed := f.fixedCross()
	need := buy + fixed
	if afterFee < need {
		return 0, false
	}
	return afterFee - need, true
}

// TriggerOK — net ≥ MinProfit и net ≥ MinSpreadBPS% от Buy.
func TriggerOK(buy, net money.NanoTON, f Fees) bool {
	if net < f.MinProfit {
		return false
	}
	minByPct := money.MulBPS(buy, f.MinSpreadBPS)
	return net >= minByPct
}

// EvalAsk — оценка по лучшему ask на выходе.
type EvalAsk struct {
	BestAsk   money.NanoTON
	Undercut  money.NanoTON
	Net       money.NanoTON
	Triggered bool
}

func EvalFromAsk(buy, bestAsk money.NanoTON, f Fees) EvalAsk {
	under := bestAsk
	if bestAsk > f.UndercutStep {
		under = bestAsk - f.UndercutStep
	}
	net, ok := NetCross(buy, bestAsk, f)
	trig := ok && TriggerOK(buy, net, f)
	return EvalAsk{BestAsk: bestAsk, Undercut: under, Net: net, Triggered: trig}
}

// EvalSales — оценка по медиане чистых sales.
type EvalSales struct {
	Median    money.NanoTON
	CleanN    int
	Net       money.NanoTON
	Triggered bool
	OK        bool // достаточно чистых sales
}

// CleanSales применяет фильтр B из CONTEXT и возвращает отсортированные чистые цены.
func CleanSales(raw []money.NanoTON, bestAsk money.NanoTON) []money.NanoTON {
	if len(raw) == 0 {
		return nil
	}
	sorted := append([]money.NanoTON(nil), raw...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	m0 := medianSorted(sorted)

	var out []money.NanoTON
	for _, p := range sorted {
		if p < money.MulBPS(m0, 7000) || p > money.MulBPS(m0, 13000) {
			continue
		}
		if bestAsk > 0 {
			if p < money.MulBPS(bestAsk, 5000) || p > money.MulBPS(bestAsk, 15000) {
				continue
			}
		}
		out = append(out, p)
	}
	return out
}

func medianSorted(sorted []money.NanoTON) money.NanoTON {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	a, b := sorted[n/2-1], sorted[n/2]
	return a/2 + b/2 + (a%2+b%2)/2 // без переполнения для средних TON
}

// MedianClean — медиана после CleanSales; ok если len >= minN.
func MedianClean(raw []money.NanoTON, bestAsk money.NanoTON, minN int) (med money.NanoTON, cleanN int, ok bool) {
	clean := CleanSales(raw, bestAsk)
	cleanN = len(clean)
	if cleanN < minN {
		return 0, cleanN, false
	}
	return medianSorted(clean), cleanN, true
}

func EvalFromSales(buy money.NanoTON, rawSales []money.NanoTON, bestAsk money.NanoTON, f Fees) EvalSales {
	med, n, ok := MedianClean(rawSales, bestAsk, f.MinCleanSales)
	if !ok {
		return EvalSales{CleanN: n, OK: false}
	}
	net, netOK := NetCross(buy, med, f)
	trig := netOK && TriggerOK(buy, net, f)
	return EvalSales{Median: med, CleanN: n, Net: net, Triggered: trig, OK: true}
}

// FullTrigger — полный сигнал: и ask, и sales.
func FullTrigger(ask EvalAsk, sales EvalSales) bool {
	return ask.Triggered && sales.OK && sales.Triggered
}
