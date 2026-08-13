package market

import (
	"context"
	"fmt"
	"strings"
)

// NormalizeMRKTToken strips quotes / Bearer / whitespace from pasted JWT.
func NormalizeMRKTToken(tok string) string {
	tok = strings.TrimSpace(tok)
	tok = strings.Trim(tok, `"'`)
	tok = strings.TrimSpace(tok)
	if len(tok) >= 7 && strings.EqualFold(tok[:7], "bearer ") {
		tok = strings.TrimSpace(tok[7:])
	}
	return tok
}

// NormalizePortalsTMA strips quotes / "tma " / Authorization prefix from pasted initData.
func NormalizePortalsTMA(tok string) string {
	tok = strings.TrimSpace(tok)
	tok = strings.Trim(tok, `"'`)
	tok = strings.TrimSpace(tok)
	lower := strings.ToLower(tok)
	if strings.HasPrefix(lower, "authorization:") {
		tok = strings.TrimSpace(tok[len("authorization:"):])
		lower = strings.ToLower(tok)
	}
	if strings.HasPrefix(lower, "tma ") {
		tok = strings.TrimSpace(tok[4:])
	}
	return strings.TrimSpace(tok)
}

// CheckAuth — лёгкий ping каталога MRKT (токен жив).
func (m *MRKT) CheckAuth(ctx context.Context) error {
	if m == nil {
		return ErrEmptyToken
	}
	_, err := m.Collections(ctx, CatalogQuery{Limit: 1})
	return err
}

// ProbeMRKT проверяет plaintext credential без мутации process-токена.
func ProbeMRKT(ctx context.Context, token string) error {
	token = NormalizeMRKTToken(token)
	if token == "" {
		return ErrEmptyToken
	}
	// MRKT принимает access_token UUID; JWT из других источников часто 401.
	if looksLikeJWT(token) {
		err := NewMRKT(Config{Auth: StaticToken(token)}).CheckAuth(ctx)
		if err != nil {
			return fmt.Errorf("%w (нужен access_token UUID из MRKT, не JWT)", err)
		}
		return nil
	}
	return NewMRKT(Config{Auth: StaticToken(token)}).CheckAuth(ctx)
}

func looksLikeJWT(tok string) bool {
	return strings.Count(tok, ".") >= 2 && strings.HasPrefix(tok, "eyJ")
}

// ProbePortals проверяет plaintext TMA.
func ProbePortals(ctx context.Context, tma string) error {
	tma = NormalizePortalsTMA(tma)
	if tma == "" {
		return ErrEmptyToken
	}
	return NewPortals(PortalsConfig{Auth: StaticToken(tma)}).CheckAuth(ctx)
}
