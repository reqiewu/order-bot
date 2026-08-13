package money

// NanoTON — минимальная единица TON (1 TON = 1e9 nanoTON).
type NanoTON uint64

const (
	TON NanoTON = 1_000_000_000

	WithdrawFee NanoTON = 300_000_000 // 0.3 TON
	TransferFee NanoTON = 250_000_000 // 0.25 TON
	GasReserve  NanoTON = 50_000_000  // 0.05 TON

	// CrossFixedFees = вывод + transfer + gas (кросс Portals↔MRKT).
	CrossFixedFees = WithdrawFee + TransferFee + GasReserve

	SellFeeBPS = 500 // 5% = 500 basis points из 10_000
	MinSpreadBPS = 500 // 5% net от BuyPrice

	DefaultMinProfit NanoTON = 100_000_000 // 0.1 TON
	DefaultUndercut  NanoTON = 10_000_000  // 0.01 TON
)

// FromTONFloat конвертирует TON (float с API) в nanoTON с округлением вниз к целым nanotons.
// Не использовать в ядре спреда — только на границе парсера.
func FromTONFloat(ton float64) NanoTON {
	if ton <= 0 {
		return 0
	}
	return NanoTON(ton * float64(TON))
}

// MulBPS умножает сумму на basis points / 10_000 (например 9500 = 95%).
func MulBPS(amount NanoTON, bps uint64) NanoTON {
	return NanoTON(uint64(amount) * bps / 10_000)
}

// PctOf возвращает bps/10000 от amount.
func PctOf(amount NanoTON, bps uint64) NanoTON {
	return MulBPS(amount, bps)
}
