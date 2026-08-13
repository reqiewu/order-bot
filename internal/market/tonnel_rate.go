package market

import (
	"context"
	"sync"
	"time"

	"github.com/reqiewu/order-bot/internal/jitter"
)

const (
	tonnelMinInterval = 400 * time.Millisecond
	tonnelJitterMax   = 200 * time.Millisecond
)

type tonnelGate struct {
	mu   sync.Mutex
	next time.Time
}

func (g *tonnelGate) wait(ctx context.Context) error {
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
	g.next = now.Add(tonnelMinInterval + jitter.UpTo(tonnelJitterMax))
	return nil
}
