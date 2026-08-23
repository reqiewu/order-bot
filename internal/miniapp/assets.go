package miniapp

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/reqiewu/order-bot/internal/assetstore"
)

func (s *Server) registerAssetRoutes() {
	// Public: only local disk cache. Sync fills files; no upstream fetch on request.
	s.mux.HandleFunc("GET /api/assets/original/{file}", s.handleAssetOriginal)
	s.mux.HandleFunc("GET /api/assets/model/{gift}/{file}", s.handleAssetModel)
}

func (s *Server) handleAssetOriginal(w http.ResponseWriter, r *http.Request) {
	if s.deps.Assets == nil {
		writeErr(w, http.StatusServiceUnavailable, "assets unavailable")
		return
	}
	name, ext := splitAssetFile(r.PathValue("file"))
	if name == "" || (ext != "png" && ext != "tgs") {
		writeErr(w, http.StatusBadRequest, "file required")
		return
	}
	size := assetPNGSize(r, ext)
	raw, _, err := s.deps.Assets.OpenOriginal(name, ext, size)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeAsset(w, ext, raw)
}

func (s *Server) handleAssetModel(w http.ResponseWriter, r *http.Request) {
	if s.deps.Assets == nil {
		writeErr(w, http.StatusServiceUnavailable, "assets unavailable")
		return
	}
	gift := strings.TrimSpace(r.PathValue("gift"))
	name, ext := splitAssetFile(r.PathValue("file"))
	if gift == "" || name == "" || (ext != "png" && ext != "tgs") {
		writeErr(w, http.StatusBadRequest, "file required")
		return
	}
	size := assetPNGSize(r, ext)
	raw, _, err := s.deps.Assets.OpenModel(gift, name, ext, size)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeAsset(w, ext, raw)
}

func splitAssetFile(file string) (name, ext string) {
	file = strings.TrimSpace(file)
	i := strings.LastIndex(file, ".")
	if i <= 0 || i == len(file)-1 {
		return file, ""
	}
	return file[:i], strings.ToLower(file[i+1:])
}

func assetPNGSize(r *http.Request, ext string) int {
	if ext != "png" {
		return 0
	}
	size := 128
	if v := strings.TrimSpace(r.URL.Query().Get("size")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			size = n
		}
	}
	return assetstore.NormalizePNGSize(size)
}

func writeAsset(w http.ResponseWriter, ext string, raw []byte) {
	ct := "application/octet-stream"
	switch ext {
	case "png":
		ct = "image/png"
	case "tgs":
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (s *Server) previewOriginal(gift string, size int) string {
	return assetstore.PublicOriginalURL(gift, "png", assetstore.NormalizePNGSize(size))
}

func (s *Server) previewModel(gift, model string, size int) string {
	return assetstore.PublicModelURL(gift, model, "png", assetstore.NormalizePNGSize(size))
}
