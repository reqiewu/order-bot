package assetstore

import (
	"context"
	"sync"
	"time"
)

// Sync fills the on-disk catalogue: backdrop colours, original PNG/TGS, every model PNG/TGS.
// Existing files are skipped. Safe to run in a background goroutine.
func (s *Store) Sync(ctx context.Context) {
	if s == nil || s.GC == nil {
		return
	}
	if err := s.refreshBackdrops(ctx); err != nil {
		if s.Log != nil {
			s.Log.Warn("assetstore backdrops", "err", err)
		}
	} else if s.Log != nil {
		s.mu.RLock()
		n := len(s.backdrops)
		s.mu.RUnlock()
		s.Log.Info("assetstore backdrops cached", "n", n)
	}

	gifts, err := s.GC.ListGifts(ctx)
	if err != nil {
		if s.Log != nil {
			s.Log.Warn("assetstore gifts list", "err", err)
		}
		return
	}

	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	var saved int
	var mu sync.Mutex
	bump := func() {
		mu.Lock()
		saved++
		n := saved
		mu.Unlock()
		if s.Log != nil && n%50 == 0 {
			s.Log.Info("assetstore progress", "files", n)
		}
	}

	for _, gift := range gifts {
		if ctx.Err() != nil {
			break
		}
		sum, err := s.GC.Gift(ctx, gift)
		if err != nil {
			if s.Log != nil {
				s.Log.Warn("assetstore gift", "gift", gift, "err", err)
			}
			continue
		}
		origJobs := []struct {
			kind, name, ext string
			size            int
		}{
			{"original", "", "tgs", 0},
		}
		for _, sz := range AllowedPNGSizes {
			origJobs = append(origJobs, struct {
				kind, name, ext string
				size            int
			}{"original", "", "png", sz})
		}
		for _, job := range origJobs {
			wg.Add(1)
			go func(gift, kind, name, ext string, size int) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					return
				}
				defer func() { <-sem }()
				if s.ensure(ctx, gift, kind, name, ext, size) {
					bump()
				}
			}(gift, job.kind, job.name, job.ext, job.size)
		}
		for _, m := range sum.Models {
			name := m.Name
			modelJobs := []struct {
				ext  string
				size int
			}{
				{"tgs", 0},
			}
			for _, sz := range AllowedPNGSizes {
				modelJobs = append(modelJobs, struct {
					ext  string
					size int
				}{"png", sz})
			}
			for _, job := range modelJobs {
				wg.Add(1)
				go func(gift, name, ext string, size int) {
					defer wg.Done()
					select {
					case sem <- struct{}{}:
					case <-ctx.Done():
						return
					}
					defer func() { <-sem }()
					if s.ensure(ctx, gift, "model", name, ext, size) {
						bump()
					}
				}(gift, name, job.ext, job.size)
			}
		}
		select {
		case <-ctx.Done():
		case <-time.After(20 * time.Millisecond):
		}
	}
	wg.Wait()
	if s.Log != nil {
		s.Log.Info("assetstore sync done", "files", saved, "gifts", len(gifts))
	}
}

func (s *Store) ensure(ctx context.Context, gift, kind, name, ext string, size int) bool {
	if kind == "original" {
		path := originalDiskPath(s.Dir, gift, ext, size)
		if exists(path) {
			return false
		}
		_, _, err := s.GetOriginal(ctx, gift, ext, size)
		return err == nil
	}
	path := modelDiskPath(s.Dir, gift, name, ext, size)
	if exists(path) {
		return false
	}
	_, _, err := s.GetModel(ctx, gift, name, ext, size)
	return err == nil
}
