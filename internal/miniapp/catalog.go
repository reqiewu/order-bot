package miniapp

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/reqiewu/order-bot/internal/giftchanges"
	"github.com/reqiewu/order-bot/internal/giftid"
	"github.com/reqiewu/order-bot/internal/market"
	"github.com/reqiewu/order-bot/internal/money"
)

func (s *Server) registerCatalogRoutes() {
	s.mux.HandleFunc("GET /api/catalog/gifts", s.withUser(s.handleCatalogGifts))
	s.mux.HandleFunc("GET /api/catalog/gifts/{gift}", s.withUser(s.handleCatalogGift))
	s.mux.HandleFunc("GET /api/catalog/gifts/{gift}/models", s.withUser(s.handleCatalogModels))
	s.mux.HandleFunc("GET /api/catalog/gifts/{gift}/backdrops", s.withUser(s.handleCatalogBackdrops))
	s.mux.HandleFunc("GET /api/catalog/gifts/{gift}/backdrops/{backdrop}", s.withUser(s.handleCatalogBackdropInfo))
	s.mux.HandleFunc("GET /api/catalog/backdrops", s.withUser(s.handleAllBackdrops))
}

func (s *Server) handleCatalogGifts(w http.ResponseWriter, r *http.Request, _ int64) {
	if s.deps.GiftChanges == nil {
		writeErr(w, http.StatusServiceUnavailable, "giftchanges unavailable")
		return
	}
	names, err := s.deps.GiftChanges.ListGifts(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}

	byFold := map[string]market.CatalogItem{}
	if s.deps.MRKT != nil {
		page, err := s.deps.MRKT.Collections(r.Context(), market.CatalogQuery{
			Limit: 5000,
			Sort:  market.CatalogSortVolumeDesc,
		})
		if err != nil {
			if s.deps.Log != nil {
				s.deps.Log.Warn("mrkt collections enrich failed", "err", err)
			}
		} else {
			for _, it := range page.Items {
				byFold[giftid.Fold(it.Name)] = it
				if it.Title != "" {
					byFold[giftid.Fold(it.Title)] = it
				}
			}
		}
	}

	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	items := make([]map[string]any, 0, len(names))
	for _, name := range names {
		if q != "" && !strings.Contains(strings.ToLower(name), q) {
			continue
		}
		row := map[string]any{
			"name":        name,
			"title":       name,
			"floor_ton":   0.0,
			"volume_ton":  0.0,
			"preview_url": s.previewOriginal(name, 128),
		}
		if it, ok := byFold[giftid.Fold(name)]; ok {
			if it.FloorNano > 0 {
				row["floor_ton"] = float64(it.FloorNano) / float64(money.TON)
			}
			if it.VolumeNano > 0 {
				row["volume_ton"] = float64(it.VolumeNano) / float64(money.TON)
			}
		}
		items = append(items, row)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"credit": "@GiftChanges",
	})
}

func (s *Server) handleCatalogGift(w http.ResponseWriter, r *http.Request, _ int64) {
	if s.deps.GiftChanges == nil {
		writeErr(w, http.StatusServiceUnavailable, "giftchanges unavailable")
		return
	}
	gift := strings.TrimSpace(r.PathValue("gift"))
	if gift == "" {
		writeErr(w, http.StatusBadRequest, "gift required")
		return
	}
	sum, err := s.deps.GiftChanges.Gift(r.Context(), gift)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if err := s.ensureBackdropColors(r.Context()); err != nil && s.deps.Log != nil {
		s.deps.Log.Warn("backdrop colours", "err", err)
	}
	models := make([]map[string]any, 0, len(sum.Models))
	for _, m := range sum.Models {
		models = append(models, map[string]any{
			"name":        m.Name,
			"rarity":      m.Rarity,
			"preview_url": s.previewModel(gift, m.Name, 128),
		})
	}
	backdrops := make([]map[string]any, 0, len(sum.Backdrops))
	for _, b := range sum.Backdrops {
		row := map[string]any{
			"name":   b.Name,
			"rarity": b.Rarity,
		}
		if c, e, p, t, ok := s.backdropColors(b.Name); ok {
			row["center_color"] = c
			row["edge_color"] = e
			row["pattern_color"] = p
			row["text_color"] = t
		}
		backdrops = append(backdrops, row)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":      firstNonEmptyStr(sum.Name, gift),
		"models":    models,
		"backdrops": backdrops,
		"credit":    "@GiftChanges",
	})
}

func (s *Server) handleCatalogModels(w http.ResponseWriter, r *http.Request, _ int64) {
	// Primary: GiftChanges rarity. Fallback MRKT names if GC fails.
	gift := strings.TrimSpace(r.PathValue("gift"))
	if gift == "" {
		writeErr(w, http.StatusBadRequest, "gift required")
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	if s.deps.GiftChanges != nil {
		sum, err := s.deps.GiftChanges.Gift(r.Context(), gift)
		if err == nil {
			items := make([]map[string]any, 0, len(sum.Models))
			for _, m := range sum.Models {
				if q != "" && !strings.Contains(strings.ToLower(m.Name), q) {
					continue
				}
				items = append(items, map[string]any{
					"name":        m.Name,
					"title":       m.Name,
					"rarity":      m.Rarity,
					"preview_url": s.previewModel(gift, m.Name, 128),
				})
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items), "source": "giftchanges"})
			return
		}
		if s.deps.Log != nil {
			s.deps.Log.Warn("giftchanges models failed, try mrkt", "err", err)
		}
	}

	if s.deps.MRKT == nil {
		writeErr(w, http.StatusServiceUnavailable, "catalog unavailable")
		return
	}
	cq := catalogQueryFromRequest(r)
	cq.Limit = catalogLimitOr(cq.Limit, 500)
	page, err := s.deps.MRKT.Models(r.Context(), gift, cq)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	items := make([]map[string]any, 0, len(page.Items))
	for _, it := range page.Items {
		items = append(items, map[string]any{
			"name":        it.Name,
			"title":       firstNonEmptyStr(it.Title, it.Name),
			"rarity":      0,
			"preview_url": s.previewModel(gift, it.Name, 128),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": page.Total, "source": "mrkt"})
}

func (s *Server) handleCatalogBackdrops(w http.ResponseWriter, r *http.Request, _ int64) {
	gift := strings.TrimSpace(r.PathValue("gift"))
	if gift == "" {
		writeErr(w, http.StatusBadRequest, "gift required")
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	model := strings.TrimSpace(r.URL.Query().Get("model"))

	if s.deps.GiftChanges != nil {
		_ = s.ensureBackdropColors(r.Context())
		sum, err := s.deps.GiftChanges.Gift(r.Context(), gift)
		if err == nil {
			items := make([]map[string]any, 0, len(sum.Backdrops))
			for _, b := range sum.Backdrops {
				if q != "" && !strings.Contains(strings.ToLower(b.Name), q) {
					continue
				}
				items = append(items, map[string]any{
					"name":   b.Name,
					"title":  b.Name,
					"rarity": b.Rarity,
				})
				if c, e, p, t, ok := s.backdropColors(b.Name); ok {
					items[len(items)-1]["center_color"] = c
					items[len(items)-1]["edge_color"] = e
					items[len(items)-1]["pattern_color"] = p
					items[len(items)-1]["text_color"] = t
				}
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items), "source": "giftchanges"})
			return
		}
	}

	if s.deps.MRKT == nil {
		writeErr(w, http.StatusServiceUnavailable, "catalog unavailable")
		return
	}
	cq := catalogQueryFromRequest(r)
	cq.Limit = catalogLimitOr(cq.Limit, 500)
	page, err := s.deps.MRKT.Backdrops(r.Context(), gift, model, cq)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	items := make([]map[string]any, 0, len(page.Items))
	for _, it := range page.Items {
		items = append(items, map[string]any{
			"name":   it.Name,
			"title":  firstNonEmptyStr(it.Title, it.Name),
			"rarity": 0,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": page.Total, "source": "mrkt"})
}

func (s *Server) handleAllBackdrops(w http.ResponseWriter, r *http.Request, _ int64) {
	if err := s.ensureBackdropColors(r.Context()); err != nil && s.deps.Log != nil {
		s.deps.Log.Warn("backdrop colours", "err", err)
	}
	list := []giftchanges.BackdropInfo{}
	if s.deps.Assets != nil {
		list = s.deps.Assets.AllBackdrops()
	}
	if len(list) == 0 && s.deps.GiftChanges != nil {
		got, err := s.deps.GiftChanges.ListBackdrops(r.Context())
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		list = got
	}
	items := make([]map[string]any, 0, len(list))
	for _, b := range list {
		items = append(items, map[string]any{
			"name":          b.Name,
			"center_color":  b.CenterColor,
			"edge_color":    b.EdgeColor,
			"pattern_color": b.PatternColor,
			"text_color":    b.TextColor,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "credit": "@GiftChanges"})
}

func (s *Server) handleCatalogBackdropInfo(w http.ResponseWriter, r *http.Request, _ int64) {
	gift := strings.TrimSpace(r.PathValue("gift"))
	backdrop := strings.TrimSpace(r.PathValue("backdrop"))
	if gift == "" || backdrop == "" {
		writeErr(w, http.StatusBadRequest, "gift and backdrop required")
		return
	}
	_ = s.ensureBackdropColors(r.Context())
	if c, e, p, t, ok := s.backdropColors(backdrop); ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"name":          backdrop,
			"center_color":  c,
			"edge_color":    e,
			"pattern_color": p,
			"text_color":    t,
		})
		return
	}
	if s.deps.GiftChanges == nil {
		writeErr(w, http.StatusServiceUnavailable, "giftchanges unavailable")
		return
	}
	info, err := s.deps.GiftChanges.BackdropInfo(r.Context(), gift, backdrop)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":          info.Name,
		"center_color":  info.CenterColor,
		"edge_color":    info.EdgeColor,
		"pattern_color": info.PatternColor,
		"text_color":    info.TextColor,
	})
}

func (s *Server) ensureBackdropColors(ctx context.Context) error {
	if s.deps.Assets != nil {
		return s.deps.Assets.EnsureBackdrops(ctx)
	}
	if s.deps.GiftChanges == nil {
		return nil
	}
	_, err := s.deps.GiftChanges.ListBackdrops(ctx)
	return err
}

func (s *Server) backdropColors(name string) (center, edge, pattern, text string, ok bool) {
	if s.deps.Assets != nil {
		if info, found := s.deps.Assets.Backdrop(name); found {
			return info.CenterColor, info.EdgeColor, info.PatternColor, info.TextColor, true
		}
	}
	return "", "", "", "", false
}

func catalogQueryFromRequest(r *http.Request) market.CatalogQuery {
	q := market.CatalogQuery{
		Query: strings.TrimSpace(r.URL.Query().Get("q")),
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			q.Offset = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			q.Limit = n
		}
	}
	return q
}

func catalogLimitOr(n, def int) int {
	if n <= 0 {
		return def
	}
	return n
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
