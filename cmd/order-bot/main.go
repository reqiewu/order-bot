package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/reqiewu/order-bot/internal/catalog"
	"github.com/reqiewu/order-bot/internal/config"
	"github.com/reqiewu/order-bot/internal/engine"
	"github.com/reqiewu/order-bot/internal/ingress"
	"github.com/reqiewu/order-bot/internal/market"
	"github.com/reqiewu/order-bot/internal/marketport"
	"github.com/reqiewu/order-bot/internal/notify"
	"github.com/reqiewu/order-bot/internal/spread"
	"github.com/reqiewu/order-bot/internal/store"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	level := slog.LevelInfo
	if cfg.LogDebug {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	st, err := store.Open(cfg.BoltPath)
	if err != nil {
		log.Error("bolt", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	if err := seedSlots(st, cfg); err != nil {
		log.Error("seed slots", "err", err)
		os.Exit(1)
	}

	slotsFn := func() []catalog.WatchSlot {
		slots, err := st.ListSlots()
		if err != nil {
			log.Warn("list slots", "err", err)
			return nil
		}
		return slots
	}

	var alerter engine.Alerter = notify.LogAlerter{Printf: func(f string, a ...any) { log.Info("paper", "text", fmt.Sprintf(f, a...)) }}
	if cfg.TelegramToken != "" && cfg.OperatorID != 0 {
		alerter = &notify.Telegram{Token: cfg.TelegramToken, ChatID: cfg.OperatorID}
	}

	mrkt := market.NewMRKT(market.Config{Auth: market.StaticToken(cfg.MRKTToken)})
	portals := market.NewPortals(market.PortalsConfig{Auth: market.NewMutableToken(cfg.PortalsTMA)})

	readers := map[string]marketport.MarketReader{
		marketport.MarketMRKT:    mrkt,
		marketport.MarketPortals: portals,
	}

	events := make(chan engine.MarketEvent, 16)
	eng := engine.New(log, spread.DefaultFees(), ingress.SalesOnDemand(readers, 0, 0), alerter, engine.NewMemoryDeduper())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-events:
				eng.Handle(ev)
			}
		}
	}()

	go (&ingress.Worker{
		Log:      log,
		Reader:   ingress.Reader{Name: marketport.MarketMRKT, Client: mrkt},
		Slots:    slotsFn,
		Interval: cfg.PollInterval,
		Out:      events,
	}).Run(ctx)

	go (&ingress.Worker{
		Log:      log,
		Reader:   ingress.Reader{Name: marketport.MarketPortals, Client: portals},
		Slots:    slotsFn,
		Interval: cfg.PollInterval,
		Out:      events,
	}).Run(ctx)

	log.Info("order-bot paper started",
		"poll", cfg.PollInterval.String(),
		"bolt", cfg.BoltPath,
		"tg", cfg.TelegramToken != "" && cfg.OperatorID != 0,
	)
	<-ctx.Done()
	log.Info("shutdown")
}

func seedSlots(st *store.Store, cfg config.Config) error {
	if cfg.WatchJSON == "" {
		return nil
	}
	var slots []catalog.WatchSlot
	if err := json.Unmarshal([]byte(cfg.WatchJSON), &slots); err != nil {
		return err
	}
	for _, s := range slots {
		if err := st.PutSlot(s); err != nil {
			return err
		}
	}
	return nil
}
