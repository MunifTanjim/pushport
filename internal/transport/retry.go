package transport

import (
	"context"
	"math/rand"
	"time"
)

type TenantSender interface {
	Send(ctx context.Context, tenantID, transportName, transportRef string, opts SendOptions, msg Message) (Result, error)
}

type RetryConfigProvider interface {
	RetryConfig(ctx context.Context) (attempts int, base, max time.Duration)
}

type RetryConfigFunc func(ctx context.Context) (attempts int, base, max time.Duration)

func (f RetryConfigFunc) RetryConfig(ctx context.Context) (int, time.Duration, time.Duration) {
	return f(ctx)
}

type RetryingSender struct {
	inner TenantSender
	cfg   RetryConfigProvider
	sleep func(ctx context.Context, d time.Duration) error
	// jitter returns a value in [0, n). rand.Int63n (the default) is safe for
	// concurrent use.
	jitter func(n int64) int64
}

func NewRetryingSender(inner TenantSender, cfg RetryConfigProvider) *RetryingSender {
	return &RetryingSender{inner: inner, cfg: cfg, sleep: ctxSleep, jitter: rand.Int63n}
}

func ctxSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func transient(res Result, err error) bool {
	if err != nil {
		return true
	}
	if res.Delivered || res.Permanent {
		return false
	}
	return res.Retryable || res.RetryAfter > 0 || res.StatusCode == 429 || res.StatusCode >= 500
}

func (r *RetryingSender) Send(ctx context.Context, tenantID, transportName, transportRef string, opts SendOptions, msg Message) (Result, error) {
	maxAttempts, base, max := r.cfg.RetryConfig(ctx)
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var res Result
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		res, err = r.inner.Send(ctx, tenantID, transportName, transportRef, opts, msg)
		if !transient(res, err) || attempt == maxAttempts {
			return res, err
		}
		// Over-budget Retry-After: return so the caller gets a 429/503 with the
		// real wait, rather than parking the goroutine on a wait we can't honor.
		if res.RetryAfter > max {
			return res, err
		}
		wait := backoff(attempt, base, max, r.jitter)
		if res.RetryAfter > wait {
			wait = res.RetryAfter
		}
		if serr := r.sleep(ctx, wait); serr != nil {
			return res, err
		}
	}
	return res, err
}

// backoff computes an exponential delay capped at max, then applies equal jitter
// so the result lands in [d/2, d] — de-synchronizing concurrent retries against
// the same provider to avoid a thundering herd on a recovering endpoint. An
// explicit Retry-After (applied by the caller) still wins over this.
func backoff(attempt int, base, max time.Duration, jitter func(n int64) int64) time.Duration {
	d := base << (attempt - 1)
	if d <= 0 || d > max {
		d = max
	}
	if d <= 1 {
		return d // too small to split; jitter would round to zero
	}
	half := int64(d) / 2
	return time.Duration(half + jitter(half+1)) // [half, 2*half] ⊆ [d/2, d]
}
