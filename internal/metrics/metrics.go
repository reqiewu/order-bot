package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/reqiewu/order-bot/internal/applog"
)

var (
	pollSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "orderbot_poll_seconds",
		Help:    "List poll duration per market (one observation per slot).",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{"market"})

	pollErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orderbot_poll_errors_total",
		Help: "List poll errors per market.",
	}, []string{"market"})

	listings = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "orderbot_listings",
		Help: "Listings returned in the last successful poll tick per market.",
	}, []string{"market"})

	tgSession = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "orderbot_tg_session",
		Help: "1 if the Telegram user session is ready, else 0.",
	})

	paperAlerts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orderbot_paper_alerts_total",
		Help: "Paper alerts delivered after dedup (one per signal).",
	}, []string{"market"})

	paperAlertErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orderbot_paper_alert_errors_total",
		Help: "Paper alert delivery failures.",
	}, []string{"market"})
)

func init() {
	prometheus.MustRegister(pollSeconds, pollErrors, listings, tgSession, paperAlerts, paperAlertErrors)
}

// ObservePoll records one List call. Call SetListings once per tick for the book size.
func ObservePoll(market string, d time.Duration, err error) {
	if market == "" {
		return
	}
	pollSeconds.WithLabelValues(market).Observe(d.Seconds())
	if err != nil {
		pollErrors.WithLabelValues(market).Inc()
	}
}

func SetListings(market string, n int) {
	if market == "" {
		return
	}
	listings.WithLabelValues(market).Set(float64(n))
}

func SetTGSession(ok bool) {
	if ok {
		tgSession.Set(1)
		return
	}
	tgSession.Set(0)
}

// ObservePaperAlert records one paper signal. err != nil counts a delivery failure.
func ObservePaperAlert(market string, err error) {
	if market == "" {
		return
	}
	if err != nil {
		paperAlertErrors.WithLabelValues(market).Inc()
		return
	}
	paperAlerts.WithLabelValues(market).Inc()
}

// Handler is the Prometheus scrape handler (separate from Mini App).
func Handler() http.Handler {
	return promhttp.Handler()
}

// Serve listens on addr until ctx is done. Empty addr is a no-op.
func Serve(ctx context.Context, addr string, log *applog.Logger) {
	if addr == "" {
		return
	}
	if log == nil {
		log = applog.Nop()
	}
	srv := &http.Server{Addr: addr, Handler: Handler()}
	go func() {
		log.Info("metrics listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("metrics failed", "err", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
}
