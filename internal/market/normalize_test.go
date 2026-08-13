package market_test

import (
	"testing"

	"github.com/reqiewu/order-bot/internal/market"
)

func TestNormalizeMRKTToken(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{`"eyJhbGciOi"`, "eyJhbGciOi"},
		{"Bearer eyJhbGciOi", "eyJhbGciOi"},
		{"  bearer   abc  ", "abc"},
	}
	for _, c := range cases {
		if got := market.NormalizeMRKTToken(c.in); got != c.want {
			t.Fatalf("NormalizeMRKTToken(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizePortalsTMA(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{`"tma user=1&hash=x"`, "user=1&hash=x"},
		{"Authorization: tma user=1", "user=1"},
		{"user=1&hash=x", "user=1&hash=x"},
	}
	for _, c := range cases {
		if got := market.NormalizePortalsTMA(c.in); got != c.want {
			t.Fatalf("NormalizePortalsTMA(%q)=%q want %q", c.in, got, c.want)
		}
	}
}
