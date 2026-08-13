package market

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// portalsMinInterval — пауза между HTTP-запросами к Portals (Radar / readers).
const portalsMinInterval = 200 * time.Millisecond

// portalsMaxRetries — доп. попытки при 429 (всего 1+N).
const portalsMaxRetries = 4

type portalsGate struct {
	mu   sync.Mutex
	next time.Time
}

// wait сериализует запросы и выдерживает минимальный интервал.
func (g *portalsGate) wait(ctx context.Context) error {
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
	g.next = now.Add(portalsMinInterval)
	return nil
}

func retryAfterDelay(header string, attempt int) time.Duration {
	if header != "" {
		if sec, err := strconv.Atoi(stringsTrimSpaceDigits(header)); err == nil && sec > 0 {
			d := time.Duration(sec) * time.Second
			if d > 30*time.Second {
				d = 30 * time.Second
			}
			return d
		}
	}
	d := 500 * time.Millisecond
	for i := 0; i < attempt; i++ {
		d *= 2
	}
	if d > 8*time.Second {
		d = 8 * time.Second
	}
	return d
}

func stringsTrimSpaceDigits(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
