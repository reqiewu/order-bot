package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/reqiewu/order-bot/internal/applog"
	"github.com/reqiewu/order-bot/internal/botcmd"
	"github.com/reqiewu/order-bot/internal/catalog"
	"github.com/reqiewu/order-bot/internal/config"
	"github.com/reqiewu/order-bot/internal/dotenv"
	"github.com/reqiewu/order-bot/internal/engine"
	"github.com/reqiewu/order-bot/internal/fx"
	"github.com/reqiewu/order-bot/internal/giftchanges"
	"github.com/reqiewu/order-bot/internal/ingress"
	"github.com/reqiewu/order-bot/internal/market"
	"github.com/reqiewu/order-bot/internal/marketport"
	"github.com/reqiewu/order-bot/internal/miniapp"
	"github.com/reqiewu/order-bot/internal/notify"
	"github.com/reqiewu/order-bot/internal/store"
	"github.com/reqiewu/order-bot/internal/tguser"
	"github.com/reqiewu/order-bot/internal/tokencrypto"
)

func main() {
	_ = dotenv.Load(".env")

	cfg, err := config.FromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	log := applog.NewLevel(cfg.LogLevel)
	defer log.Sync()

	st, err := store.Open(cfg.BoltPath)
	if err != nil {
		log.Error("bolt open failed", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	if keyRaw := strings.TrimSpace(os.Getenv("TOKEN_ENCRYPTION_KEY")); keyRaw != "" {
		key, err := tokencrypto.ParseKey(keyRaw)
		if err != nil {
			log.Error("TOKEN_ENCRYPTION_KEY invalid", "err", err)
			os.Exit(1)
		}
		st.SetEncKey(key)
	} else {
		log.Warn("TOKEN_ENCRYPTION_KEY empty — Mini App token save disabled until set")
	}

	if err := seedSlots(st, cfg); err != nil {
		log.Error("seed slots failed", "err", err)
		os.Exit(1)
	}

	rt, _ := st.GetRuntime()
	fees := miniapp.FeesFromRuntime(rt)

	mrktTok := market.NewMutableMRKT(cfg.MRKTToken)
	portalsTok := market.NewMutableToken(cfg.PortalsTMA)
	if st.HasMRKTToken() {
		if t, err := st.MRKTToken(); err == nil && t != "" {
			// Prefer bolt only if it still works; otherwise keep env.
			probeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := market.ProbeMRKT(probeCtx, t); err == nil {
				mrktTok.SetMRKT(t)
				log.Info("MRKT token loaded from bolt")
			} else {
				log.Warn("MRKT token in bolt is dead — using env if set", "err", err)
				if mrktTok.Get() == "" {
					mrktTok.SetMRKT(t) // still set so Mini App status reflects stored value
				}
			}
			cancel()
		}
	}
	if st.HasPortalsTMA() {
		if t, err := st.PortalsTMA(); err == nil && t != "" {
			probeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := market.ProbePortals(probeCtx, t); err == nil {
				portalsTok.Set(t)
				log.Info("Portals TMA loaded from bolt")
			} else {
				log.Warn("Portals TMA in bolt is dead — using env if set", "err", err)
				if portalsTok.Get() == "" {
					portalsTok.Set(t)
				}
			}
			cancel()
		}
	}

	getgemsKey := market.NormalizeGetgemsAPIKey(cfg.GetgemsAPIKey)
	if st.HasGetgemsAPIKey() {
		if t, err := st.GetgemsAPIKey(); err == nil && t != "" {
			probeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := market.ProbeGetgems(probeCtx, t); err == nil {
				getgemsKey = t
				log.Info("Getgems API key loaded from bolt")
			} else {
				log.Warn("Getgems API key in bolt is dead — using env if set", "err", err)
				if getgemsKey == "" {
					getgemsKey = t
				}
			}
			cancel()
		}
	}
	tonnelInit := market.NormalizePortalsTMA(cfg.TonnelInitData)
	if st.HasTonnelInitData() {
		if t, err := st.TonnelInitData(); err == nil && t != "" {
			probeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := market.ProbeTonnelInitData(probeCtx, t); err == nil {
				tonnelInit = t
				log.Info("Tonnel initData loaded from bolt")
			} else {
				log.Warn("Tonnel initData in bolt is dead — using env if set", "err", err)
				if tonnelInit == "" {
					tonnelInit = t
				}
			}
			cancel()
		}
	}

	mrkt := market.NewMRKT(market.Config{Auth: mrktTok})
	portals := market.NewPortals(market.PortalsConfig{Auth: portalsTok})
	// Always construct so Mini App can enable Getgems without restart.
	getgems := market.NewGetgems(market.GetgemsConfig{APIKey: getgemsKey})

	readers := map[string]marketport.MarketReader{
		marketport.MarketMRKT:    mrkt,
		marketport.MarketPortals: portals,
		marketport.MarketGetgems: getgems,
	}
	var tonnel *market.Tonnel
	if !cfg.TonnelDisabled {
		tonnel = market.NewTonnel(market.TonnelConfig{
			BaseURL:  cfg.TonnelBaseURL,
			InitData: tonnelInit,
		})
		readers[marketport.MarketTonnel] = tonnel
	}

	var tgMarket *market.Telegram
	var tgUser *tguser.Client
	if !cfg.TelegramUserOff && cfg.TelegramAPIID > 0 && cfg.TelegramAPIHash != "" {
		var err error
		tgUser, err = tguser.New(tguser.Config{
			APIID:       cfg.TelegramAPIID,
			APIHash:     cfg.TelegramAPIHash,
			SessionPath: cfg.TelegramSession,
		})
		if err != nil {
			log.Warn("Telegram user client", "err", err)
		} else if !tgUser.HasSessionFile() {
			log.Info("telegram market off — no session; go run ./cmd/tg-login")
			tgUser = nil
		} else {
			tgMarket = market.NewTelegram(market.TelegramConfig{User: tgUser})
			readers[marketport.MarketTelegram] = tgMarket
		}
	} else if cfg.TelegramUserOff {
		log.Info("telegram market off")
	} else {
		log.Info("telegram market off — no TELEGRAM_API_ID / TELEGRAM_API_HASH")
	}

	probeCtx, cancelProbe := context.WithTimeout(context.Background(), 15*time.Second)
	checkTokens(probeCtx, log, mrktTok, portalsTok, mrkt, portals, getgems, tonnel, nil)
	cancelProbe()

	slotsFn := func() []catalog.WatchSlot {
		slots, err := st.ListSlots()
		if err != nil {
			log.Warn("list slots", "err", err)
			return nil
		}
		return slots
	}
	intervalFn := func() time.Duration { return st.PollInterval() }

	rates := fx.NewCMC()
	logAlert := notify.LogAlerter{Log: log, Rates: rates}
	var alerter engine.Alerter = logAlert
	if cfg.TelegramToken != "" && cfg.OperatorID != 0 {
		alerter = notify.Multi{Alerts: []engine.Alerter{
			logAlert,
			&notify.Telegram{Token: cfg.TelegramToken, ChatID: cfg.OperatorID, Rates: rates},
		}}
	}

	events := make(chan engine.MarketEvent, 16)
	eng := engine.New(log, fees, ingress.SalesOnDemand(readers, 0, 0), alerter, engine.NewMemoryDeduper())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if tgUser != nil {
		go func() {
			if err := tgUser.Run(ctx); err != nil && ctx.Err() == nil {
				log.Warn("Telegram MTProto stopped", "err", err)
			}
		}()
		waitCtx, cancelWait := context.WithTimeout(ctx, 45*time.Second)
		if err := tgUser.WaitReady(waitCtx, 45*time.Second); err != nil {
			log.Warn("Telegram MTProto not ready — venue skipped this boot", "err", err)
			delete(readers, marketport.MarketTelegram)
			tgMarket = nil
		} else {
			log.Info("Telegram MTProto ready (Gift Marketplace asks)")
			probeCtx2, cancel2 := context.WithTimeout(ctx, 15*time.Second)
			if err := tgMarket.CheckAuth(probeCtx2); err != nil {
				log.Warn("Telegram getStarGifts failed", "err", err)
			} else {
				log.Info("Telegram getStarGifts ok")
			}
			cancel2()
		}
		cancelWait()
	}
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
		Log: log, Reader: ingress.Reader{Name: marketport.MarketMRKT, Client: mrkt},
		Slots: slotsFn, IntervalFn: intervalFn, Out: events,
	}).Run(ctx)
	go (&ingress.Worker{
		Log: log, Reader: ingress.Reader{Name: marketport.MarketPortals, Client: portals},
		Slots: slotsFn, IntervalFn: intervalFn, Out: events,
	}).Run(ctx)
	go (&ingress.Worker{
		Log: log, Reader: ingress.Reader{Name: marketport.MarketGetgems, Client: getgems},
		Slots: slotsFn, IntervalFn: intervalFn, Out: events,
	}).Run(ctx)
	if tonnel != nil {
		go (&ingress.Worker{
			Log: log, Reader: ingress.Reader{Name: marketport.MarketTonnel, Client: tonnel},
			Slots: slotsFn, IntervalFn: intervalFn, Out: events,
		}).Run(ctx)
	}
	if tgMarket != nil {
		go (&ingress.Worker{
			Log: log, Reader: ingress.Reader{Name: marketport.MarketTelegram, Client: tgMarket},
			Slots: slotsFn, IntervalFn: intervalFn, Out: events,
		}).Run(ctx)
	}

	miniCfg, err := miniapp.ConfigFromEnv(cfg.TelegramToken, cfg.OperatorID)
	if err != nil {
		log.Error("miniapp config", "err", err)
		os.Exit(1)
	}
	if miniCfg.Enabled() {
		srv := miniapp.New(miniapp.Deps{
			Config:      miniCfg,
			Store:       st,
			Log:         log,
			GiftChanges: giftchanges.NewGiftChanges(),
			MRKT:        mrkt,
			Portals:     portals,
			Getgems:     getgems,
			Tonnel:      tonnel,
			LiveMRKT:    mrktTok.Get,
			LivePortals: portalsTok.Get,
			LiveGetgems: getgems.APIKey,
			LiveTonnel: func() string {
				if tonnel == nil {
					return ""
				}
				return tonnel.InitData()
			},
			Hooks: miniapp.TokenHooks{
				OnMRKT: func(t string) {
					mrktTok.SetMRKT(t)
					log.Info("MRKT token updated via Mini App")
				},
				OnPortals: func(t string) {
					portalsTok.Set(t)
					log.Info("Portals token updated via Mini App")
				},
				OnGetgems: func(t string) {
					getgems.SetAPIKey(t)
					log.Info("Getgems token updated via Mini App")
				},
				OnTonnel: func(t string) {
					if tonnel != nil {
						tonnel.SetInitData(t)
						log.Info("Tonnel token updated via Mini App")
					}
				},
				OnRuntime: func(r store.Runtime) {
					eng.SetFees(miniapp.FeesFromRuntime(r))
					log.Info("runtime updated", "poll_sec", r.PollIntervalSec, "min_profit_nano", r.MinProfitNano)
				},
			},
		})
		httpSrv := &http.Server{Addr: miniCfg.Addr, Handler: srv.Handler()}
		go func() {
			log.Info("miniapp listening", "addr", miniCfg.Addr, "static", miniCfg.StaticDir)
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("miniapp failed", "err", err)
			}
		}()
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = httpSrv.Shutdown(shutdownCtx)
		}()
	}

	if cfg.TelegramToken != "" && cfg.OperatorID != 0 {
		webApp := strings.TrimSpace(os.Getenv("MINIAPP_PUBLIC_URL"))
		go func() {
			b := botcmd.New(botcmd.Config{
				Token:      cfg.TelegramToken,
				OperatorID: cfg.OperatorID,
				WebAppURL:  webApp,
				Log:        log,
			})
			if err := b.Run(ctx); err != nil && ctx.Err() == nil {
				log.Error("command bot stopped", "err", err)
			}
		}()
	}

	log.Info("order-bot paper started",
		"poll", intervalFn().String(),
		"bolt", cfg.BoltPath,
		"mrkt", mrkt.Enabled(),
		"portals", portals.Enabled(),
		"getgems", getgems.Enabled(),
		"tonnel", tonnel != nil && tonnel.Enabled(),
		"telegram", tgMarket != nil,
		"miniapp", miniCfg.Enabled(),
		"log", cfg.LogLevel,
	)
	<-ctx.Done()
	log.Info("shutdown")
}

func checkTokens(
	ctx context.Context,
	log *applog.Logger,
	mrktTok, portalsTok *market.MutableToken,
	mrkt *market.MRKT,
	portals *market.Portals,
	getgems *market.Getgems,
	tonnel *market.Tonnel,
	_ *market.Telegram, // probed after MTProto ready
) {
	if mrktTok.Get() == "" {
		log.Info("mrkt off — no MRKT_TOKEN")
	} else if err := mrkt.CheckAuth(ctx); err != nil {
		log.Warn("MRKT token dead — update in Mini App", "err", err)
	} else {
		log.Info("MRKT token ok")
	}

	if portalsTok.Get() == "" {
		log.Info("portals off — no PORTALS_TOKEN")
	} else if err := portals.CheckAuth(ctx); err != nil {
		log.Warn("Portals token dead — update in Mini App", "err", err)
	} else {
		log.Info("Portals token ok")
	}

	if !getgems.HasAPIKey() {
		log.Info("getgems off — no GETGEMS_TOKEN")
	} else if err := getgems.CheckAuth(ctx); err != nil {
		log.Warn("Getgems token dead", "err", err)
	} else {
		log.Info("Getgems token ok")
	}

	if tonnel == nil {
		log.Info("tonnel off")
	} else if !tonnel.HasInitData() {
		log.Info("tonnel off — no TONNEL_TOKEN")
	} else if err := tonnel.CheckAuth(ctx); err != nil {
		log.Warn("Tonnel token dead", "err", err)
	} else {
		log.Info("Tonnel token ok")
	}
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
