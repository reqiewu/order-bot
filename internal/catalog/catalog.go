package catalog

import (
	"fmt"
	"strings"

	"github.com/reqiewu/order-bot/internal/giftid"
	"github.com/reqiewu/order-bot/internal/money"
)

// ModelBG — ключ матча коллекция+model+backdrop.
type ModelBG struct {
	Collection string
	Model      string
	Backdrop   string
}

func (k ModelBG) Key() string {
	return giftid.Fold(k.Collection) + "\x00" + giftid.Fold(k.Model) + "\x00" + giftid.Fold(k.Backdrop)
}

func (k ModelBG) String() string {
	return fmt.Sprintf("%s / %s / %s", k.Collection, k.Model, k.Backdrop)
}

// WatchSlot — whitelist-слот (коллекция обязательна).
type WatchSlot struct {
	ID         string `json:"id"`
	Collection string `json:"collection"`
	Model      string `json:"model"`
	Backdrop   string `json:"backdrop"`
}

func (s WatchSlot) Valid() bool {
	return strings.TrimSpace(s.Collection) != ""
}

// Lot — нормализованный лот в nanoTON.
type Lot struct {
	Market     string
	ListingID  string
	ModelBG    ModelBG
	Price      money.NanoTON
	Number     *int
	URL        string
}

// SalePoint — продажа в nanoTON.
type SalePoint struct {
	Market  string
	ModelBG ModelBG
	Price   money.NanoTON
	AtUnix  int64
}
