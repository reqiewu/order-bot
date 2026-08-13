package botcmd_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/reqiewu/order-bot/internal/botcmd"
)

func TestDescriptions(t *testing.T) {
	t.Parallel()
	if botcmd.BotDescription() == "" {
		t.Fatal("empty description")
	}
	if n := utf8.RuneCountInString(botcmd.BotShortDescription()); n == 0 || n > 120 {
		t.Fatalf("short description length %d", n)
	}
	if !strings.Contains(botcmd.WelcomeHTML(), "order-bot") {
		t.Fatal("welcome missing brand")
	}
}
