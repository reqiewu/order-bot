package market

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// PortalsMarketConfig — first-party GET /api/market/config.
type PortalsMarketConfig struct {
	Commission    float64 // доля, напр. 0.02
	OfferFee      float64
	WithdrawalFee float64 // flat TON, напр. 0.3
}

type portalsMarketConfigJSON struct {
	Commission    json.RawMessage `json:"commission"`
	OfferFee      json.RawMessage `json:"offer_fee"`
	WithdrawalFee json.RawMessage `json:"withdrawal_fee"`
}

// MarketConfig читает live комиссии Portals (без auth).
func (p *Portals) MarketConfig(ctx context.Context) (PortalsMarketConfig, error) {
	if p == nil {
		return PortalsMarketConfig{}, fmt.Errorf("market/portals: nil client")
	}
	raw, err := p.get(ctx, "/api/market/config", nil, false)
	if err != nil {
		return PortalsMarketConfig{}, err
	}
	var body portalsMarketConfigJSON
	if err := json.Unmarshal(raw, &body); err != nil {
		return PortalsMarketConfig{}, fmt.Errorf("market/portals: decode market config: %w", err)
	}
	commission, err := parsePortalsFloat(body.Commission)
	if err != nil {
		return PortalsMarketConfig{}, fmt.Errorf("market/portals: commission: %w", err)
	}
	offer, err := parsePortalsFloat(body.OfferFee)
	if err != nil {
		return PortalsMarketConfig{}, fmt.Errorf("market/portals: offer_fee: %w", err)
	}
	withdraw, err := parsePortalsFloat(body.WithdrawalFee)
	if err != nil {
		return PortalsMarketConfig{}, fmt.Errorf("market/portals: withdrawal_fee: %w", err)
	}
	return PortalsMarketConfig{
		Commission:    commission,
		OfferFee:      offer,
		WithdrawalFee: withdraw,
	}, nil
}

func parsePortalsFloat(raw json.RawMessage) (float64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}
	var asNum float64
	if err := json.Unmarshal(raw, &asNum); err == nil {
		return asNum, nil
	}
	var asStr string
	if err := json.Unmarshal(raw, &asStr); err != nil {
		return 0, fmt.Errorf("want number or string, got %s", truncate(raw, 40))
	}
	v, err := strconv.ParseFloat(asStr, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}