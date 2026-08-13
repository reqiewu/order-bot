package jitter

import (
	"context"
	"math/rand"
	"time"
)

// UpTo returns a random duration in [0, max].
func UpTo(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(max) + 1))
}

// Between returns a random duration in [min, max].
func Between(min, max time.Duration) time.Duration {
	if max < min {
		min, max = max, min
	}
	if max <= min {
		return min
	}
	return min + UpTo(max-min)
}

// Around returns base scaled by (1 ± frac). frac=0.15 → ±15%.
func Around(base time.Duration, frac float64) time.Duration {
	if base <= 0 {
		return base
	}
	if frac <= 0 {
		return base
	}
	if frac > 0.9 {
		frac = 0.9
	}
	span := time.Duration(float64(base) * frac)
	return base - span + UpTo(2*span)
}

// Sleep waits Between(min,max) or until ctx done.
func Sleep(ctx context.Context, min, max time.Duration) error {
	d := Between(min, max)
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
