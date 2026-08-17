package assetstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/reqiewu/order-bot/internal/applog"
	"github.com/reqiewu/order-bot/internal/giftchanges"
	"github.com/reqiewu/order-bot/internal/giftid"
)

// Store — on-disk TGS/PNG cache + backdrop colour table.
// Miss → GiftChanges, then write so the next hit skips the API.
type Store struct {
	Dir string
	GC  *giftchanges.GiftChanges
	Log *applog.Logger

	mu        sync.RWMutex
	backdrops map[string]giftchanges.BackdropInfo
}

func New(dir string, gc *giftchanges.GiftChanges, log *applog.Logger) *Store {
	s := &Store{Dir: dir, GC: gc, Log: log, backdrops: map[string]giftchanges.BackdropInfo{}}
	_ = os.MkdirAll(dir, 0o755)
	_ = s.loadBackdropsFile()
	return s
}

func PublicOriginalURL(gift, ext string, size int) string {
	u := "/api/assets/original/" + url.PathEscape(SafeSegment(gift)+"."+ext)
	if ext == "png" && size > 0 {
		u += fmt.Sprintf("?size=%d", size)
	}
	return u
}

func PublicModelURL(gift, model, ext string, size int) string {
	u := "/api/assets/model/" + url.PathEscape(SafeSegment(gift)) + "/" + url.PathEscape(SafeSegment(model)+"."+ext)
	if ext == "png" && size > 0 {
		u += fmt.Sprintf("?size=%d", size)
	}
	return u
}

func SafeSegment(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '/' || r == '\\' {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' || r == '\'' {
			b.WriteRune(r)
		}
	}
	out := strings.ReplaceAll(b.String(), "..", "")
	if out == "" {
		return "x"
	}
	return out
}

func (s *Store) AllBackdrops() []giftchanges.BackdropInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]giftchanges.BackdropInfo, 0, len(s.backdrops))
	for _, b := range s.backdrops {
		out = append(out, b)
	}
	return out
}

func (s *Store) Backdrop(name string) (giftchanges.BackdropInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.backdrops[giftid.Fold(name)]
	return info, ok
}

func (s *Store) EnsureBackdrops(ctx context.Context) error {
	s.mu.RLock()
	n := len(s.backdrops)
	s.mu.RUnlock()
	if n > 0 {
		return nil
	}
	return s.refreshBackdrops(ctx)
}

func (s *Store) refreshBackdrops(ctx context.Context) error {
	if s.GC == nil {
		return fmt.Errorf("assetstore: no giftchanges client")
	}
	list, err := s.GC.ListBackdrops(ctx)
	if err != nil {
		return err
	}
	s.setBackdrops(list)
	return s.saveBackdropsFile(list)
}

func (s *Store) setBackdrops(list []giftchanges.BackdropInfo) {
	next := make(map[string]giftchanges.BackdropInfo, len(list))
	for _, b := range list {
		next[giftid.Fold(b.Name)] = b
	}
	s.mu.Lock()
	s.backdrops = next
	s.mu.Unlock()
}

func (s *Store) backdropsPath() string {
	return filepath.Join(s.Dir, "backdrops.json")
}

func (s *Store) loadBackdropsFile() error {
	raw, err := os.ReadFile(s.backdropsPath())
	if err != nil {
		return err
	}
	var list []giftchanges.BackdropInfo
	if err := json.Unmarshal(raw, &list); err != nil {
		return err
	}
	s.setBackdrops(list)
	return nil
}

func (s *Store) saveBackdropsFile(list []giftchanges.BackdropInfo) error {
	raw, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return os.WriteFile(s.backdropsPath(), raw, 0o644)
}

func originalDiskPath(dir, gift, ext string, size int) string {
	g := SafeSegment(gift)
	name := g
	if ext == "png" && size > 0 && size != 128 {
		name += fmt.Sprintf("@%d", size)
	}
	return filepath.Join(dir, "original", name+"."+ext)
}

func modelDiskPath(dir, gift, model, ext string, size int) string {
	g := SafeSegment(gift)
	m := SafeSegment(model)
	if ext == "png" && size > 0 && size != 128 {
		m += fmt.Sprintf("@%d", size)
	}
	return filepath.Join(dir, "model", g, m+"."+ext)
}

func (s *Store) GetOriginal(ctx context.Context, gift, ext string, size int) ([]byte, string, error) {
	path := originalDiskPath(s.Dir, gift, ext, size)
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		return raw, contentType(ext), nil
	}
	if s.GC == nil {
		return nil, "", fmt.Errorf("assetstore: miss and no client")
	}
	raw, ct, err := s.GC.FetchOriginal(ctx, gift, ext, size)
	if err != nil {
		return nil, "", err
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, raw, 0o644)
	if ct == "" {
		ct = contentType(ext)
	}
	return raw, ct, nil
}

func (s *Store) GetModel(ctx context.Context, gift, model, ext string, size int) ([]byte, string, error) {
	path := modelDiskPath(s.Dir, gift, model, ext, size)
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		return raw, contentType(ext), nil
	}
	if s.GC == nil {
		return nil, "", fmt.Errorf("assetstore: miss and no client")
	}
	raw, ct, err := s.GC.FetchModel(ctx, gift, model, ext, size)
	if err != nil {
		return nil, "", err
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, raw, 0o644)
	if ct == "" {
		ct = contentType(ext)
	}
	return raw, ct, nil
}

func contentType(ext string) string {
	switch ext {
	case "png":
		return "image/png"
	case "tgs":
		return "application/octet-stream"
	case "json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func exists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Size() > 0
}
