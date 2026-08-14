package market

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/tg"

	"github.com/reqiewu/order-bot/internal/giftid"
	"github.com/reqiewu/order-bot/internal/marketport"
	"github.com/reqiewu/order-bot/internal/tguser"
)

const (
	telegramPageSize = 50
	telegramMaxPages = 3 // ~150 raw → cap ListLimit
)

// TelegramConfig — Gift Marketplace через MTProto user session.
type TelegramConfig struct {
	User      *tguser.Client
	MaxPages  int
	PageSize  int
	ListLimit int
}

// Telegram — asks из payments.getResaleStarGifts (только TON-цены).
// RecentSales пока пустой (история — отдельным этапом).
type Telegram struct {
	user      *tguser.Client
	maxPages  int
	pageSize  int
	listLimit int

	mu         sync.Mutex
	giftByFold map[string]int64 // fold(title) → base gift_id
	giftAt     time.Time
}

// NewTelegram wraps a ready-to-Run tguser.Client.
func NewTelegram(cfg TelegramConfig) *Telegram {
	maxPages := cfg.MaxPages
	if maxPages <= 0 {
		maxPages = telegramMaxPages
	}
	pageSize := cfg.PageSize
	if pageSize <= 0 {
		pageSize = telegramPageSize
	}
	listLimit := cfg.ListLimit
	if listLimit <= 0 {
		listLimit = marketport.DefaultListLimit
	}
	return &Telegram{
		user:       cfg.User,
		maxPages:   maxPages,
		pageSize:   pageSize,
		listLimit:  listLimit,
		giftByFold: map[string]int64{},
	}
}

func (t *Telegram) Enabled() bool { return t != nil && t.user != nil }

// CheckAuth — лёгкий ping: getStarGifts.
func (t *Telegram) CheckAuth(ctx context.Context) error {
	api, err := t.user.API(ctx)
	if err != nil {
		return err
	}
	_, err = api.PaymentsGetStarGifts(ctx, 0)
	return err
}

func (t *Telegram) List(ctx context.Context, watch marketport.WatchItem) ([]marketport.Listing, error) {
	if strings.TrimSpace(watch.Collection) == "" {
		return nil, fmt.Errorf("market/telegram: empty collection")
	}
	api, err := t.user.API(ctx)
	if err != nil {
		return nil, err
	}
	giftID, err := t.resolveGiftID(ctx, api, watch.Collection)
	if err != nil {
		return nil, err
	}
	var out []marketport.Listing
	offset := ""
	for page := 0; page < t.maxPages && len(out) < t.listLimit; page++ {
		res, err := api.PaymentsGetResaleStarGifts(ctx, &tg.PaymentsGetResaleStarGiftsRequest{
			SortByPrice: true,
			GiftID:      giftID,
			Offset:      offset,
			Limit:       t.pageSize,
		})
		if err != nil {
			return nil, fmt.Errorf("market/telegram: getResaleStarGifts: %w", err)
		}
		if res == nil || len(res.Gifts) == 0 {
			break
		}
		for _, g := range res.Gifts {
			u, ok := g.(*tg.StarGiftUnique)
			if !ok || u == nil {
				continue
			}
			view := uniqueToView(u, watch.Collection)
			lot, ok := normalizeTelegramUnique(view)
			if !ok {
				continue
			}
			if !matchTelegramWatch(lot, watch) {
				continue
			}
			out = append(out, lot)
			if len(out) >= t.listLimit {
				break
			}
		}
		next, has := res.GetNextOffset()
		if !has || strings.TrimSpace(next) == "" {
			break
		}
		offset = next
	}
	return marketport.TakeCheapest(out, t.listLimit), nil
}

// RecentSales — stub: история маркетплейса TG ещё не подключена.
func (t *Telegram) RecentSales(ctx context.Context, like marketport.Listing, limit int) ([]marketport.Sale, error) {
	_ = ctx
	_ = like
	_ = limit
	return nil, nil
}

func (t *Telegram) resolveGiftID(ctx context.Context, api *tg.Client, collection string) (int64, error) {
	want := giftid.Fold(collection)
	t.mu.Lock()
	if id, ok := t.giftByFold[want]; ok && time.Since(t.giftAt) < 30*time.Minute {
		t.mu.Unlock()
		return id, nil
	}
	t.mu.Unlock()

	raw, err := api.PaymentsGetStarGifts(ctx, 0)
	if err != nil {
		return 0, fmt.Errorf("market/telegram: getStarGifts: %w", err)
	}
	gifts, ok := raw.(*tg.PaymentsStarGifts)
	if !ok || gifts == nil {
		return 0, fmt.Errorf("market/telegram: unexpected getStarGifts type %T", raw)
	}
	next := map[string]int64{}
	var matched int64
	for _, g := range gifts.Gifts {
		sg, ok := g.(*tg.StarGift)
		if !ok || sg == nil {
			continue
		}
		title, _ := sg.GetTitle()
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}
		fold := giftid.Fold(title)
		next[fold] = sg.ID
		if fold == want || giftid.SameCollection(title, collection) {
			matched = sg.ID
		}
	}
	t.mu.Lock()
	t.giftByFold = next
	t.giftAt = time.Now()
	t.mu.Unlock()
	if matched == 0 {
		return 0, fmt.Errorf("market/telegram: gift_id not found for %q", collection)
	}
	return matched, nil
}

func uniqueToView(u *tg.StarGiftUnique, collectionHint string) tgUniqueView {
	v := tgUniqueView{
		ID:         u.ID,
		Title:      strings.TrimSpace(u.Title),
		Slug:       strings.TrimSpace(u.Slug),
		Num:        u.Num,
		Collection: strings.TrimSpace(collectionHint),
	}
	if v.Collection == "" {
		v.Collection = v.Title
	}
	amounts, _ := u.GetResellAmount()
	views := make([]tgAmountView, 0, len(amounts))
	for _, a := range amounts {
		switch x := a.(type) {
		case *tg.StarsTonAmount:
			if x != nil {
				views = append(views, tgAmountView{IsTON: true, Amount: x.Amount})
			}
		case *tg.StarsAmount:
			// Stars — пропускаем (paper в nanoTON).
		}
	}
	v.TonNano = tonPriceFromAmounts(views)
	for _, attr := range u.Attributes {
		switch a := attr.(type) {
		case *tg.StarGiftAttributeModel:
			if a != nil {
				v.Model = a.Name
			}
		case *tg.StarGiftAttributeBackdrop:
			if a != nil {
				v.Backdrop = a.Name
			}
		case *tg.StarGiftAttributePattern:
			if a != nil {
				v.Symbol = a.Name
			}
		}
	}
	return v
}
