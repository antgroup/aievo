package errors

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Policy controls retry behaviour.
//
// Backoff: t_n = min(MaxBackoff, BaseBackoff * 2^n) + jitter
// Jitter:  uniform in [0, BaseBackoff) added to each delay to prevent
//          thundering-herd retries against a recovering LLM provider.
type Policy struct {
	MaxAttempts int           // total attempts including the first; 0 means 3
	BaseBackoff time.Duration // 0 means 250ms
	MaxBackoff  time.Duration // 0 means 5s
}

// DefaultPolicy is the recommended baseline for LLM and tool retries.
var DefaultPolicy = Policy{MaxAttempts: 3, BaseBackoff: 250 * time.Millisecond, MaxBackoff: 5 * time.Second}

// Retry calls fn until it returns nil, ctx is cancelled, or fn returns a
// non-retryable error (see IsRetryable). The returned error is the last
// observed error from fn, or ctx.Err() if the context was cancelled mid-wait.
func Retry(ctx context.Context, p Policy, fn func(context.Context) error) error {
	if p.MaxAttempts == 0 {
		p.MaxAttempts = 3
	}
	if p.BaseBackoff == 0 {
		p.BaseBackoff = 250 * time.Millisecond
	}
	if p.MaxBackoff == 0 {
		p.MaxBackoff = 5 * time.Second
	}
	var lastErr error
	for attempt := 0; attempt < p.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err
		if !IsRetryable(err) {
			return err
		}
		if attempt == p.MaxAttempts-1 {
			break
		}
		delay := backoff(p, attempt)
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
	return lastErr
}

func backoff(p Policy, attempt int) time.Duration {
	exp := float64(p.BaseBackoff) * math.Pow(2, float64(attempt))
	if exp > float64(p.MaxBackoff) {
		exp = float64(p.MaxBackoff)
	}
	jitter := time.Duration(rand.Int63n(int64(p.BaseBackoff)))
	return time.Duration(exp) + jitter
}
