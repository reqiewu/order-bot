package market

import (
	"context"
	"sync"
	"time"

	"github.com/reqiewu/order-bot/internal/jitter"
)

const (
	mrktMinInterval = 150 * time.Millisecond
	mrktJitterMax   = 250 * time.Millisecond
)

type mrktGate struct {
	mu   sync.Mutex
	next time.Time
}

func (g *mrktGate) wait(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	if now.Before(g.next) {
		wait := g.next.Sub(now)
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			now = time.Now()
		}
	}
	g.next = now.Add(mrktMinInterval + jitter.UpTo(mrktJitterMax))
	return nil
}
