package market

import (
	"context"
	"errors"
	"strings"
)

// FallbackToken tries primary, then secondary TokenProvider (e.g. reader → owner).
type FallbackToken struct {
	Primary   TokenProvider
	Secondary TokenProvider
}

func (f FallbackToken) Token(ctx context.Context) (string, error) {
	var errs []string
	if f.Primary != nil {
		tok, err := f.Primary.Token(ctx)
		if err == nil && strings.TrimSpace(tok) != "" {
			return tok, nil
		}
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	if f.Secondary != nil {
		tok, err := f.Secondary.Token(ctx)
		if err == nil && strings.TrimSpace(tok) != "" {
			return tok, nil
		}
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) == 0 {
		return "", errors.New("market: empty token")
	}
	return "", errors.New(strings.Join(errs, "; "))
}

// OwnerToken loads MRKT token for a fixed telegram id from Users store.
type OwnerTokenFunc func(ctx context.Context) (string, error)

func (f OwnerTokenFunc) Token(ctx context.Context) (string, error) {
	if f == nil {
		return "", errors.New("market: nil owner token func")
	}
	tok, err := f(ctx)
	if err != nil {
		return "", err
	}
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return "", errors.New("market: empty owner MRKT token")
	}
	return tok, nil
}
