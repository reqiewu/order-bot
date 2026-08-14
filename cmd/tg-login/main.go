package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/reqiewu/order-bot/internal/dotenv"
	"github.com/reqiewu/order-bot/internal/tguser"
)

func main() {
	_ = dotenv.Load(".env")
	apiID, _ := strconv.Atoi(strings.TrimSpace(os.Getenv("TELEGRAM_API_ID")))
	apiHash := strings.TrimSpace(os.Getenv("TELEGRAM_API_HASH"))
	session := strings.TrimSpace(os.Getenv("TELEGRAM_SESSION_PATH"))
	if session == "" {
		session = "data/tg.session"
	}
	phone := strings.TrimSpace(os.Getenv("TELEGRAM_PHONE"))
	password := os.Getenv("TELEGRAM_2FA_PASSWORD") // may be empty

	if apiID == 0 || apiHash == "" {
		fmt.Fprintln(os.Stderr, "Set TELEGRAM_API_ID and TELEGRAM_API_HASH from https://my.telegram.org/apps")
		os.Exit(1)
	}
	if err := tguser.Login(context.Background(), tguser.Config{
		APIID:       apiID,
		APIHash:     apiHash,
		SessionPath: session,
	}, phone, password); err != nil {
		fmt.Fprintf(os.Stderr, "login failed: %v\n", err)
		os.Exit(1)
	}
}
