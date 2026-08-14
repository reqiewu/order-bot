package miniapp

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/reqiewu/order-bot/internal/applog"
	"github.com/reqiewu/order-bot/internal/catalog"
	"github.com/reqiewu/order-bot/internal/giftchanges"
	"github.com/reqiewu/order-bot/internal/market"
	"github.com/reqiewu/order-bot/internal/money"
	"github.com/reqiewu/order-bot/internal/spread"
	"github.com/reqiewu/order-bot/internal/store"
)

// TokenHooks обновляет live credentials после записи в store.
type TokenHooks struct {
	OnMRKT    func(token string)
	OnPortals func(tma string)
	OnGetgems func(apiKey string)
	OnTonnel  func(initData string)
	OnRuntime func(r store.Runtime)
}

// Deps — зависимости HTTP Mini App.
type Deps struct {
	Config      Config
	Store       *store.Store
	Log         *applog.Logger
	Hooks       TokenHooks
	GiftChanges *giftchanges.GiftChanges
	MRKT        *market.MRKT
	Portals     *market.Portals
	Getgems     *market.Getgems
	Tonnel      *market.Tonnel
	// Live token getters — то, чем процесс реально ходит в API (env и/или bolt).
	LiveMRKT    func() string
	LivePortals func() string
	LiveGetgems func() string
	LiveTonnel  func() string
}

// Server — API + SPA.
type Server struct {
	deps Deps
	mux  *http.ServeMux
	once sync.Once
}

func New(deps Deps) *Server {
	if deps.GiftChanges == nil {
		deps.GiftChanges = giftchanges.NewGiftChanges()
	}
	s := &Server{deps: deps, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.cors(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.HandleFunc("GET /api/settings/tokens", s.withUser(s.getTokens))
	s.mux.HandleFunc("PUT /api/settings/tokens", s.withUser(s.putTokens))
	s.mux.HandleFunc("POST /api/settings/tokens/probe", s.withUser(s.probeTokens))
	s.mux.HandleFunc("GET /api/settings/runtime", s.withUser(s.getRuntime))
	s.mux.HandleFunc("PUT /api/settings/runtime", s.withUser(s.putRuntime))
	s.mux.HandleFunc("GET /api/slots", s.withUser(s.listSlots))
	s.mux.HandleFunc("POST /api/slots", s.withUser(s.postSlot))
	s.mux.HandleFunc("DELETE /api/slots", s.withUser(s.deleteSlot))
	s.registerCatalogRoutes()

	if dir := strings.TrimSpace(s.deps.Config.StaticDir); dir != "" {
		s.mux.Handle("/", spaFileServer(dir))
	}
}

type userHandler func(w http.ResponseWriter, r *http.Request, uid int64)

func (s *Server) withUser(next userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := s.authenticate(r)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, err.Error())
			return
		}
		if s.deps.Config.OperatorID != 0 && uid != s.deps.Config.OperatorID {
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r, uid)
	}
}

func (s *Server) authenticate(r *http.Request) (int64, error) {
	cfg := s.deps.Config
	if cfg.DevMode {
		if raw := strings.TrimSpace(r.Header.Get("X-Dev-Telegram-Id")); raw != "" {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id <= 0 {
				return 0, ErrInvalidAuth
			}
			return id, nil
		}
		if cfg.DevBypassID > 0 {
			return cfg.DevBypassID, nil
		}
	}
	initData := strings.TrimSpace(r.Header.Get("X-Telegram-Init-Data"))
	if initData == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "tma ") {
			initData = strings.TrimSpace(auth[4:])
		}
	}
	u, err := ValidateInitData(initData, cfg.BotToken)
	if err != nil {
		return 0, err
	}
	return u.ID, nil
}

func (s *Server) getTokens(w http.ResponseWriter, r *http.Request, _ int64) {
	out := s.tokenPresence()
	if r.URL.Query().Get("probe") == "1" {
		status := s.probeLiveTokens(r.Context())
		out["mrkt_ok"] = status.MRKTOk
		out["portals_ok"] = status.PortalsOk
		out["getgems_ok"] = status.GetgemsOk
		out["tonnel_ok"] = status.TonnelOk
		if status.MRKTErr != "" {
			out["mrkt_error"] = status.MRKTErr
		}
		if status.PortalsErr != "" {
			out["portals_error"] = status.PortalsErr
		}
		if status.GetgemsErr != "" {
			out["getgems_error"] = status.GetgemsErr
		}
		if status.TonnelErr != "" {
			out["tonnel_error"] = status.TonnelErr
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) tokenPresence() map[string]any {
	liveMRKT := strings.TrimSpace(s.liveMRKT())
	livePortals := strings.TrimSpace(s.livePortals())
	liveGetgems := strings.TrimSpace(s.liveGetgems())
	liveTonnel := strings.TrimSpace(s.liveTonnel())
	storedMRKT := s.deps.Store.HasMRKTToken()
	storedPortals := s.deps.Store.HasPortalsTMA()
	storedGetgems := s.deps.Store.HasGetgemsAPIKey()
	storedTonnel := s.deps.Store.HasTonnelInitData()
	return map[string]any{
		"mrkt_set":       liveMRKT != "" || storedMRKT,
		"portals_set":    livePortals != "" || storedPortals,
		"getgems_set":    liveGetgems != "" || storedGetgems,
		"tonnel_set":     liveTonnel != "" || storedTonnel,
		"mrkt_live":      liveMRKT != "",
		"portals_live":   livePortals != "",
		"getgems_live":   liveGetgems != "",
		"tonnel_live":    liveTonnel != "",
		"mrkt_stored":    storedMRKT,
		"portals_stored": storedPortals,
		"getgems_stored": storedGetgems,
		"tonnel_stored":  storedTonnel,
	}
}

func (s *Server) liveMRKT() string {
	if s.deps.LiveMRKT != nil {
		return s.deps.LiveMRKT()
	}
	return ""
}

func (s *Server) livePortals() string {
	if s.deps.LivePortals != nil {
		return s.deps.LivePortals()
	}
	return ""
}

func (s *Server) liveGetgems() string {
	if s.deps.LiveGetgems != nil {
		return s.deps.LiveGetgems()
	}
	if s.deps.Getgems != nil {
		return s.deps.Getgems.APIKey()
	}
	return ""
}

func (s *Server) liveTonnel() string {
	if s.deps.LiveTonnel != nil {
		return s.deps.LiveTonnel()
	}
	if s.deps.Tonnel != nil {
		return s.deps.Tonnel.InitData()
	}
	return ""
}

type tokenProbeStatus struct {
	MRKTOk     bool
	PortalsOk  bool
	GetgemsOk  bool
	TonnelOk   bool
	MRKTErr    string
	PortalsErr string
	GetgemsErr string
	TonnelErr  string
}

// probeLiveTokens проверяет credentials процесса (то, чем реально ходит ingress).
func (s *Server) probeLiveTokens(ctx context.Context) tokenProbeStatus {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	var st tokenProbeStatus

	if s.liveMRKT() == "" {
		st.MRKTErr = "empty"
	} else if s.deps.MRKT == nil {
		st.MRKTErr = "mrkt client unavailable"
	} else if err := s.deps.MRKT.CheckAuth(ctx); err != nil {
		st.MRKTErr = err.Error()
	} else {
		st.MRKTOk = true
	}

	if s.livePortals() == "" {
		st.PortalsErr = "empty"
	} else if s.deps.Portals == nil {
		st.PortalsErr = "portals client unavailable"
	} else if err := s.deps.Portals.CheckAuth(ctx); err != nil {
		st.PortalsErr = err.Error()
	} else {
		st.PortalsOk = true
	}

	if s.liveGetgems() == "" {
		st.GetgemsErr = "empty"
	} else if s.deps.Getgems == nil {
		st.GetgemsErr = "getgems client unavailable"
	} else if err := s.deps.Getgems.CheckAuth(ctx); err != nil {
		st.GetgemsErr = err.Error()
	} else {
		st.GetgemsOk = true
	}

	if s.liveTonnel() == "" {
		st.TonnelErr = "empty"
	} else if s.deps.Tonnel == nil {
		st.TonnelErr = "tonnel client unavailable"
	} else if err := market.ProbeTonnelInitData(ctx, s.liveTonnel()); err != nil {
		st.TonnelErr = err.Error()
	} else {
		st.TonnelOk = true
	}
	return st
}

func (s *Server) probeTokens(w http.ResponseWriter, r *http.Request, _ int64) {
	var body struct {
		MRKTToken      *string `json:"mrkt_token"`
		PortalsTMA     *string `json:"portals_tma"`
		GetgemsAPIKey  *string `json:"getgems_api_key"`
		TonnelInitData *string `json:"tonnel_initdata"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	out := s.tokenPresence()
	out["ok"] = true

	if body.MRKTToken == nil && body.PortalsTMA == nil && body.GetgemsAPIKey == nil && body.TonnelInitData == nil {
		st := s.probeLiveTokens(ctx)
		out["mrkt_ok"] = st.MRKTOk
		out["portals_ok"] = st.PortalsOk
		out["getgems_ok"] = st.GetgemsOk
		out["tonnel_ok"] = st.TonnelOk
		if st.MRKTErr != "" {
			out["mrkt_error"] = st.MRKTErr
		}
		if st.PortalsErr != "" {
			out["portals_error"] = st.PortalsErr
		}
		if st.GetgemsErr != "" {
			out["getgems_error"] = st.GetgemsErr
		}
		if st.TonnelErr != "" {
			out["tonnel_error"] = st.TonnelErr
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	if body.MRKTToken != nil {
		tok := market.NormalizeMRKTToken(*body.MRKTToken)
		if tok == "" {
			out["mrkt_ok"] = false
			out["mrkt_error"] = "empty"
		} else if err := market.ProbeMRKT(ctx, tok); err != nil {
			out["mrkt_ok"] = false
			out["mrkt_error"] = err.Error()
		} else {
			out["mrkt_ok"] = true
		}
	}
	if body.PortalsTMA != nil {
		tok := market.NormalizePortalsTMA(*body.PortalsTMA)
		if tok == "" {
			out["portals_ok"] = false
			out["portals_error"] = "empty"
		} else if err := market.ProbePortals(ctx, tok); err != nil {
			out["portals_ok"] = false
			out["portals_error"] = err.Error()
		} else {
			out["portals_ok"] = true
		}
	}
	if body.GetgemsAPIKey != nil {
		tok := market.NormalizeGetgemsAPIKey(*body.GetgemsAPIKey)
		if tok == "" {
			out["getgems_ok"] = false
			out["getgems_error"] = "empty"
		} else if err := market.ProbeGetgems(ctx, tok); err != nil {
			out["getgems_ok"] = false
			out["getgems_error"] = err.Error()
		} else {
			out["getgems_ok"] = true
		}
	}
	if body.TonnelInitData != nil {
		tok := market.NormalizePortalsTMA(*body.TonnelInitData)
		if tok == "" {
			out["tonnel_ok"] = false
			out["tonnel_error"] = "empty"
		} else if err := market.ProbeTonnelInitData(ctx, tok); err != nil {
			out["tonnel_ok"] = false
			out["tonnel_error"] = err.Error()
		} else {
			out["tonnel_ok"] = true
		}
	}

	writeJSON(w, http.StatusOK, out)
}

func (s *Server) putTokens(w http.ResponseWriter, r *http.Request, _ int64) {
	var body struct {
		MRKTToken      *string `json:"mrkt_token"`
		PortalsTMA     *string `json:"portals_tma"`
		GetgemsAPIKey  *string `json:"getgems_api_key"`
		TonnelInitData *string `json:"tonnel_initdata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.MRKTToken == nil && body.PortalsTMA == nil && body.GetgemsAPIKey == nil && body.TonnelInitData == nil {
		writeErr(w, http.StatusBadRequest, "nothing to save")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	out := map[string]any{"ok": true}

	if body.MRKTToken != nil {
		tok := market.NormalizeMRKTToken(*body.MRKTToken)
		if tok == "" {
			writeErr(w, http.StatusBadRequest, "empty mrkt_token")
			return
		}
		if err := market.ProbeMRKT(ctx, tok); err != nil {
			writeErr(w, http.StatusBadRequest, "mrkt token invalid: "+err.Error())
			return
		}
		if err := s.deps.Store.PutMRKTToken(tok); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if s.deps.Hooks.OnMRKT != nil {
			s.deps.Hooks.OnMRKT(tok)
		}
		out["mrkt_ok"] = true
	}
	if body.PortalsTMA != nil {
		tma := market.NormalizePortalsTMA(*body.PortalsTMA)
		if tma == "" {
			writeErr(w, http.StatusBadRequest, "empty portals_tma")
			return
		}
		if err := market.ProbePortals(ctx, tma); err != nil {
			writeErr(w, http.StatusBadRequest, "portals tma invalid: "+err.Error())
			return
		}
		if err := s.deps.Store.PutPortalsTMA(tma); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if s.deps.Hooks.OnPortals != nil {
			s.deps.Hooks.OnPortals(tma)
		}
		out["portals_ok"] = true
	}
	if body.GetgemsAPIKey != nil {
		key := market.NormalizeGetgemsAPIKey(*body.GetgemsAPIKey)
		if key == "" {
			writeErr(w, http.StatusBadRequest, "empty getgems_api_key")
			return
		}
		if err := market.ProbeGetgems(ctx, key); err != nil {
			writeErr(w, http.StatusBadRequest, "getgems api key invalid: "+err.Error())
			return
		}
		if err := s.deps.Store.PutGetgemsAPIKey(key); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if s.deps.Hooks.OnGetgems != nil {
			s.deps.Hooks.OnGetgems(key)
		}
		out["getgems_ok"] = true
	}
	if body.TonnelInitData != nil {
		initData := market.NormalizePortalsTMA(*body.TonnelInitData)
		if initData == "" {
			writeErr(w, http.StatusBadRequest, "empty tonnel_initdata")
			return
		}
		if err := market.ProbeTonnelInitData(ctx, initData); err != nil {
			writeErr(w, http.StatusBadRequest, "tonnel initData invalid: "+err.Error())
			return
		}
		if err := s.deps.Store.PutTonnelInitData(initData); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if s.deps.Hooks.OnTonnel != nil {
			s.deps.Hooks.OnTonnel(initData)
		}
		out["tonnel_ok"] = true
	}
	for k, v := range s.tokenPresence() {
		out[k] = v
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getRuntime(w http.ResponseWriter, _ *http.Request, _ int64) {
	rt, err := s.deps.Store.GetRuntime()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"poll_interval_sec": rt.PollIntervalSec,
		"min_profit_ton":    float64(rt.MinProfitNano) / float64(money.TON),
		"min_spread_bps":    rt.MinSpreadBPS,
		"min_spread_pct":    float64(rt.MinSpreadBPS) / 100.0,
	})
}

func (s *Server) putRuntime(w http.ResponseWriter, r *http.Request, _ int64) {
	var body struct {
		PollIntervalSec *int     `json:"poll_interval_sec"`
		MinProfitTON    *float64 `json:"min_profit_ton"`
		MinSpreadBPS    *int     `json:"min_spread_bps"`
		MinSpreadPct    *float64 `json:"min_spread_pct"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	rt, err := s.deps.Store.GetRuntime()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if body.PollIntervalSec != nil {
		rt.PollIntervalSec = *body.PollIntervalSec
	}
	if body.MinProfitTON != nil {
		rt.MinProfitNano = int64(money.FromTONFloat(*body.MinProfitTON))
	}
	if body.MinSpreadBPS != nil {
		rt.MinSpreadBPS = *body.MinSpreadBPS
	} else if body.MinSpreadPct != nil {
		rt.MinSpreadBPS = int(*body.MinSpreadPct * 100)
	}
	if err := s.deps.Store.PutRuntime(rt); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.deps.Hooks.OnRuntime != nil {
		s.deps.Hooks.OnRuntime(rt)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) listSlots(w http.ResponseWriter, _ *http.Request, _ int64) {
	slots, err := s.deps.Store.ListSlots()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if slots == nil {
		slots = []catalog.WatchSlot{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"slots": slots})
}

func (s *Server) postSlot(w http.ResponseWriter, r *http.Request, _ int64) {
	var slot catalog.WatchSlot
	if err := json.NewDecoder(r.Body).Decode(&slot); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	slot.Collection = strings.TrimSpace(slot.Collection)
	slot.Model = strings.TrimSpace(slot.Model)
	slot.Backdrop = strings.TrimSpace(slot.Backdrop)
	if !slot.Valid() {
		writeErr(w, http.StatusBadRequest, "collection required")
		return
	}
	if err := s.deps.Store.PutSlot(slot); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	log := s.deps.Log
	if log == nil {
		log = applog.Nop()
	}
	log.InfoTree("watch slot added",
		applog.KV{K: "collection", V: slotAny(slot.Collection)},
		applog.KV{K: "model", V: slotAny(slot.Model)},
		applog.KV{K: "background", V: slotAny(slot.Backdrop)},
	)
	slots, _ := s.deps.Store.ListSlots()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "slots": slots})
}

func (s *Server) deleteSlot(w http.ResponseWriter, r *http.Request, _ int64) {
	var body struct {
		ID         string `json:"id"`
		Collection string `json:"collection"`
		Model      string `json:"model"`
		Backdrop   string `json:"backdrop"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	id := strings.TrimSpace(body.ID)
	if id == "" {
		id = strings.TrimSpace(body.Collection) + "|" + strings.TrimSpace(body.Model) + "|" + strings.TrimSpace(body.Backdrop)
	}
	if err := s.deps.Store.DeleteSlot(id); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	coll, model, bg := body.Collection, body.Model, body.Backdrop
	if strings.TrimSpace(coll) == "" {
		coll, model, bg = splitSlotID(id)
	}
	log := s.deps.Log
	if log == nil {
		log = applog.Nop()
	}
	log.InfoTree("watch slot removed",
		applog.KV{K: "collection", V: slotAny(coll)},
		applog.KV{K: "model", V: slotAny(model)},
		applog.KV{K: "background", V: slotAny(bg)},
	)
	slots, _ := s.deps.Store.ListSlots()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "slots": slots})
}

func slotAny(s string) string {
	if strings.TrimSpace(s) == "" {
		return "any"
	}
	return s
}

func splitSlotID(id string) (collection, model, backdrop string) {
	parts := strings.SplitN(id, "|", 3)
	if len(parts) > 0 {
		collection = parts[0]
	}
	if len(parts) > 1 {
		model = parts[1]
	}
	if len(parts) > 2 {
		backdrop = parts[2]
	}
	return
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Telegram-Init-Data, Authorization, X-Dev-Telegram-Id")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func spaFileServer(dir string) http.Handler {
	fs := http.Dir(dir)
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		index := filepath.Join(dir, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})
}

// FeesFromRuntime строит spread.Fees из Runtime.
func FeesFromRuntime(rt store.Runtime) spread.Fees {
	f := spread.DefaultFees()
	f.MinProfit = money.NanoTON(rt.MinProfitNano)
	f.MinSpreadBPS = uint64(rt.MinSpreadBPS)
	return f
}
