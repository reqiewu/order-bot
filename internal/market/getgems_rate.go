package market

import (
	"context"
	"sync"
	"time"

	"github.com/reqiewu/order-bot/internal/jitter"
)

const (
	getgemsMinInterval = 80 * time.Millisecond
	getgemsJitterMax   = 40 * time.Millisecond
)

type getgemsGate struct {
	mu   sync.Mutex
	next time.Time
}

func (g *getgemsGate) wait(ctx context.Context) error {
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
	g.next = now.Add(getgemsMinInterval + jitter.UpTo(getgemsJitterMax))
	return nil
}
