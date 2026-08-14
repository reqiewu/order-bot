package marketport_test

import (
	"testing"

	"github.com/reqiewu/order-bot/internal/marketport"
)

func TestTakeCheapest(t *testing.T) {
	in := []marketport.Listing{
		{ID: "a", Price: 3},
		{ID: "b", Price: 1},
		{ID: "c", Price: 2},
		{ID: "d", Price: 0.5},
	}
	got := marketport.TakeCheapest(in, 2)
	if len(got) != 2 || got[0].ID != "d" || got[1].ID != "b" {
		t.Fatalf("%+v", got)
	}
	if len(in) != 4 {
		t.Fatal("input mutated")
	}
}
