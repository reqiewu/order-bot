package tmasession

import (
	"context"
	"strings"
	"time"

	"github.com/reqiewu/order-bot/internal/applog"
	"github.com/reqiewu/order-bot/internal/market"
	"github.com/reqiewu/order-bot/internal/store"
	"github.com/reqiewu/order-bot/internal/tguser"
)

const (
	botPortals = "portals_market_bot"
	appPortals = "market"
	botTonnel  = "tonnel_network_bot"
	appTonnel  = "gift"
	botMRKT    = "mrkt"
	appMRKT    = "app"

	refreshEvery = 20 * time.Minute
	mintGap      = 400 * time.Millisecond
)

// Targets — live-токены, которые наполняем из user-сессии Telegram.
type Targets struct {
	User    *tguser.Client
	Store   *store.Store
	Log     *applog.Logger
	MRKT    *market.MutableToken
	Portals *market.MutableToken
	Tonnel  *market.Tonnel
}

// Loop обновляет TMA-токены по таймеру (initData протухает ~час).
func (t *Targets) Loop(ctx context.Context) {
	if t == nil || t.User == nil {
		return
	}
	tick := time.NewTicker(refreshEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			t.Refresh(ctx)
		}
	}
}

// Refresh выпускает Mini App initData из MTProto и кладёт в live + bolt.
func (t *Targets) Refresh(ctx context.Context) {
	if t == nil || t.User == nil {
		return
	}
	log := t.Log
	if log == nil {
		log = applog.Nop()
	}

	t.mintPortals(ctx, log)
	if err := sleep(ctx, mintGap); err != nil {
		return
	}
	t.mintTonnel(ctx, log)
	if err := sleep(ctx, mintGap); err != nil {
		return
	}
	t.mintMRKT(ctx, log)
}

func (t *Targets) mintPortals(ctx context.Context, log *applog.Logger) {
	if t.Portals == nil {
		return
	}
	initData, err := t.User.RequestInitData(ctx, botPortals, appPortals)
	if err != nil {
		log.Warn("Portals initData from session failed — update Telegram session", "err", err)
		return
	}
	t.Portals.Set(initData)
	persist(t.Store, log, "portals", "tma", initData)
	if err := market.ProbePortals(ctx, initData); err != nil {
		log.Warn("Portals session token set, probe failed", "err", err)
		return
	}
	log.Info("Portals token from Telegram session")
}

func (t *Targets) mintTonnel(ctx context.Context, log *applog.Logger) {
	if t.Tonnel == nil {
		return
	}
	initData, err := t.User.RequestInitData(ctx, botTonnel, appTonnel)
	if err != nil {
		log.Warn("Tonnel initData from session failed — update Telegram session", "err", err)
		return
	}
	t.Tonnel.SetInitData(initData)
	persist(t.Store, log, "tonnel", "tma", initData)
	if err := market.ProbeTonnelInitData(ctx, initData); err != nil {
		log.Warn("Tonnel session token set, probe failed", "err", err)
		return
	}
	log.Info("Tonnel token from Telegram session")
}

func (t *Targets) mintMRKT(ctx context.Context, log *applog.Logger) {
	if t.MRKT == nil {
		return
	}
	if tok := t.MRKT.Get(); tok != "" {
		if err := market.ProbeMRKT(ctx, tok); err == nil {
			return
		}
		log.Warn("MRKT token dead — remint from session")
	}
	initData, err := t.User.RequestInitData(ctx, botMRKT, appMRKT)
	if err != nil {
		log.Warn("MRKT initData from session failed — update Telegram session", "err", err)
		return
	}
	tok, err := market.ExchangeMRKTAuth(ctx, initData)
	if err != nil {
		log.Warn("MRKT auth exchange failed", "err", err)
		return
	}
	t.MRKT.SetMRKT(tok)
	persist(t.Store, log, "mrkt", "uuid", tok)
	if err := market.ProbeMRKT(ctx, tok); err != nil {
		log.Warn("MRKT session token set, probe failed", "err", err)
		return
	}
	log.Info("MRKT token from Telegram session")
}

func persist(st *store.Store, log *applog.Logger, name, kind, val string) {
	if st == nil {
		return
	}
	var err error
	switch name {
	case "portals":
		err = st.PutPortalsTMA(val)
	case "tonnel":
		err = st.PutTonnelInitData(val)
	case "mrkt":
		err = st.PutMRKTToken(val)
	}
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "encryption key not set") {
		log.Debug("skip persist " + name + " — no TOKEN_ENCRYPTION_KEY")
		return
	}
	log.Warn("persist "+name+" from session", "kind", kind, "err", err)
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
