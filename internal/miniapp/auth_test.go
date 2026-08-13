package miniapp

import (
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestValidateInitData(t *testing.T) {
	t.Parallel()
	botToken := "123456:ABC-DEF"
	userJSON := `{"id":42,"first_name":"Test"}`
	authDate := strconv.FormatInt(time.Now().Unix(), 10)
	vals := url.Values{}
	vals.Set("auth_date", authDate)
	vals.Set("user", userJSON)
	var pairs []string
	for k, vv := range vals {
		if len(vv) == 0 {
			continue
		}
		pairs = append(pairs, k+"="+vv[0])
	}
	sort.Strings(pairs)
	dataCheck := strings.Join(pairs, "\n")
	secretKey := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	hash := hex.EncodeToString(hmacSHA256(secretKey, []byte(dataCheck)))
	vals.Set("hash", hash)
	initData := vals.Encode()

	u, err := ValidateInitData(initData, botToken)
	if err != nil {
		t.Fatalf("valid init data: %v", err)
	}
	if u.ID != 42 {
		t.Fatalf("user id: %d", u.ID)
	}

	_, err = ValidateInitData(initData, "wrong-token")
	if err != ErrInvalidAuth {
		t.Fatalf("want invalid auth, got %v", err)
	}
}
