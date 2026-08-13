package miniapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrMissingAuth   = errors.New("miniapp: missing telegram init data")
	ErrInvalidAuth   = errors.New("miniapp: invalid telegram init data")
	ErrAuthExpired   = errors.New("miniapp: telegram init data expired")
)

const maxAuthAge = 24 * time.Hour

// TelegramUser — subset of WebApp initData user JSON.
type TelegramUser struct {
	ID int64 `json:"id"`
}

// ValidateInitData checks Telegram WebApp initData HMAC (Bot API).
func ValidateInitData(initData, botToken string) (TelegramUser, error) {
	initData = strings.TrimSpace(initData)
	if initData == "" {
		return TelegramUser{}, ErrMissingAuth
	}
	if botToken == "" {
		return TelegramUser{}, fmt.Errorf("miniapp: bot token required for auth")
	}
	vals, err := url.ParseQuery(initData)
	if err != nil {
		return TelegramUser{}, ErrInvalidAuth
	}
	hash := vals.Get("hash")
	if hash == "" {
		return TelegramUser{}, ErrInvalidAuth
	}
	var pairs []string
	for k, vv := range vals {
		if k == "hash" {
			continue
		}
		if len(vv) == 0 {
			continue
		}
		pairs = append(pairs, k+"="+vv[0])
	}
	sort.Strings(pairs)
	dataCheck := strings.Join(pairs, "\n")

	secretKey := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	expected := hex.EncodeToString(hmacSHA256(secretKey, []byte(dataCheck)))
	if !hmac.Equal([]byte(expected), []byte(hash)) {
		return TelegramUser{}, ErrInvalidAuth
	}
	if raw := vals.Get("auth_date"); raw != "" {
		sec, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			authTime := time.Unix(sec, 0)
			if time.Since(authTime) > maxAuthAge {
				return TelegramUser{}, ErrAuthExpired
			}
		}
	}
	userRaw := vals.Get("user")
	if userRaw == "" {
		return TelegramUser{}, ErrInvalidAuth
	}
	userDecoded, err := url.QueryUnescape(userRaw)
	if err != nil {
		userDecoded = userRaw
	}
	var u struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(userDecoded), &u); err != nil || u.ID <= 0 {
		return TelegramUser{}, ErrInvalidAuth
	}
	return TelegramUser{ID: u.ID}, nil
}

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	_, _ = m.Write(data)
	return m.Sum(nil)
}
