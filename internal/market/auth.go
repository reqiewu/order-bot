package market

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
)

// TokenProvider выдаёт Authorization-токен MRKT / Portals.
type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

var (
	// ErrEmptyToken — credential не задан.
	ErrEmptyToken = errors.New("market: empty token")
)

// StaticToken возвращает фиксированный токен из конфигурации.
type StaticToken string

func (s StaticToken) Token(context.Context) (string, error) {
	if s == "" {
		return "", ErrEmptyToken
	}
	return string(s), nil
}

// StaticTokenFromEnv читает статический токен из указанной переменной окружения.
func StaticTokenFromEnv(key string) StaticToken {
	return StaticToken(os.Getenv(key))
}

// PortalsTMAFromEnv читает TMA initData Portals из env (process reader).
// Значение может быть с префиксом "tma " или без — адаптер нормализует заголовок.
func PortalsTMAFromEnv(key string) StaticToken {
	return StaticTokenFromEnv(key)
}

// MutableToken — TokenProvider, который можно обновить без рестарта.
type MutableToken struct {
	mu  sync.RWMutex
	tok string
}

// NewMutableToken создаёт MutableToken с начальным значением (может быть пустым).
func NewMutableToken(initial string) *MutableToken {
	return &MutableToken{tok: NormalizePortalsTMA(initial)}
}

// NewMutableMRKT — как NewMutableToken, но нормализует JWT.
func NewMutableMRKT(initial string) *MutableToken {
	return &MutableToken{tok: NormalizeMRKTToken(initial)}
}

func (m *MutableToken) Token(context.Context) (string, error) {
	if m == nil {
		return "", errors.New("market: nil MutableToken")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.tok == "" {
		return "", ErrEmptyToken
	}
	return m.tok, nil
}

// Set обновляет токен (нормализует как Portals TMA — для MRKT используй SetMRKT).
func (m *MutableToken) Set(tok string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tok = NormalizePortalsTMA(tok)
}

// SetMRKT обновляет JWT с MRKT-нормализацией.
func (m *MutableToken) SetMRKT(tok string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tok = NormalizeMRKTToken(tok)
}

// Get возвращает текущий plaintext; пустая строка если не задан.
func (m *MutableToken) Get() string {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tok
}

// UnauthorizedError — API отклонил учётные данные (например, HTTP 401).
type UnauthorizedError struct {
	Cause error
}

func (e *UnauthorizedError) Error() string {
	if e.Cause == nil {
		return "market: unauthorized"
	}
	return fmt.Sprintf("market: unauthorized: %v", e.Cause)
}

func (e *UnauthorizedError) Unwrap() error { return e.Cause }

// IsUnauthorized сообщает, является ли err (или обёртка) UnauthorizedError.
func IsUnauthorized(err error) bool {
	var u *UnauthorizedError
	return errors.As(err, &u)
}

