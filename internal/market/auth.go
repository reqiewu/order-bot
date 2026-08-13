package market

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

// TokenProvider выдаёт Authorization-токен MRKT / Portals.
type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

// StaticToken возвращает фиксированный токен из конфигурации.
type StaticToken string

func (s StaticToken) Token(context.Context) (string, error) {
	if s == "" {
		return "", errors.New("market: empty static MRKT token")
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

// MutableToken — TokenProvider, который можно обновить без рестарта (Portals TMA).
type MutableToken struct {
	mu  sync.RWMutex
	tok string
}

// NewMutableToken создаёт MutableToken с начальным значением (может быть пустым).
func NewMutableToken(initial string) *MutableToken {
	return &MutableToken{tok: normalizePortalsTMA(initial)}
}

func (m *MutableToken) Token(context.Context) (string, error) {
	if m == nil {
		return "", errors.New("market: nil MutableToken")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.tok == "" {
		return "", errors.New("market: empty Portals TMA")
	}
	return m.tok, nil
}

// Set обновляет токен в памяти (нормализует пробелы/кавычки).
func (m *MutableToken) Set(tok string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tok = normalizePortalsTMA(tok)
}

// Get возвращает текущий plaintext (для persist); пустая строка если не задан.
func (m *MutableToken) Get() string {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tok
}

func normalizePortalsTMA(tok string) string {
	tok = strings.TrimSpace(tok)
	tok = strings.Trim(tok, `"'`)
	return strings.TrimSpace(tok)
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
