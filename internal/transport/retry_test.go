package transport

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubRetryCfg struct {
	attempts  int
	base, max time.Duration
}

func (s stubRetryCfg) RetryConfig(context.Context) (int, time.Duration, time.Duration) {
	return s.attempts, s.base, s.max
}

func TestRetryingSenderUsesProviderAttempts(t *testing.T) {
	inner := &scriptedSender{
		results: []Result{
			{StatusCode: 500},
			{StatusCode: 500},
			{StatusCode: 500},
		},
	}
	rs := NewRetryingSender(inner, stubRetryCfg{attempts: 3, base: time.Millisecond, max: time.Millisecond})
	rs.sleep = func(context.Context, time.Duration) error { return nil }
	_, _ = rs.Send(context.Background(), "t", "apns", "ref", SendOptions{}, Message{})
	if inner.calls != 3 {
		t.Fatalf("expected 3 attempts from provider, got %d", inner.calls)
	}
}

type scriptedSender struct {
	results []Result
	errs    []error
	calls   int
}

func (s *scriptedSender) Send(_ context.Context, _, _, _ string, _ SendOptions, _ Message) (Result, error) {
	i := s.calls
	s.calls++
	var err error
	if i < len(s.errs) {
		err = s.errs[i]
	}
	var res Result
	if i < len(s.results) {
		res = s.results[i]
	}
	return res, err
}

func newNoSleepRetrier(inner TenantSender, attempts int) *RetryingSender {
	r := NewRetryingSender(inner, stubRetryCfg{attempts: attempts, base: time.Millisecond, max: time.Millisecond})
	r.sleep = func(context.Context, time.Duration) error { return nil }
	return r
}

func TestRetrySucceedsAfterTransient(t *testing.T) {
	inner := &scriptedSender{
		results: []Result{{StatusCode: 503}, {Delivered: true, StatusCode: 200}},
	}
	r := newNoSleepRetrier(inner, 3)
	res, err := r.Send(context.Background(), "t", "apns", "ref", SendOptions{}, Message{})
	if err != nil || !res.Delivered {
		t.Fatalf("expected delivered, got res=%+v err=%v", res, err)
	}
	if inner.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", inner.calls)
	}
}

func TestRetryStopsOnPermanent(t *testing.T) {
	inner := &scriptedSender{results: []Result{{Permanent: true, StatusCode: 410}}}
	r := newNoSleepRetrier(inner, 3)
	res, _ := r.Send(context.Background(), "t", "apns", "ref", SendOptions{}, Message{})
	if !res.Permanent || inner.calls != 1 {
		t.Fatalf("permanent should not retry: res=%+v calls=%d", res, inner.calls)
	}
}

func TestBackoffEqualJitterBounds(t *testing.T) {
	base, max := 100*time.Millisecond, 10*time.Second
	minJitter := func(int64) int64 { return 0 }
	maxJitter := func(n int64) int64 {
		if n <= 1 {
			return 0
		}
		return n - 1
	}
	for attempt := 1; attempt <= 8; attempt++ {
		d := base << (attempt - 1)
		if d <= 0 || d > max {
			d = max
		}
		lo := d / 2
		if got := backoff(attempt, base, max, minJitter); got != lo {
			t.Fatalf("attempt %d min-jitter: want %v, got %v", attempt, lo, got)
		}
		if got := backoff(attempt, base, max, maxJitter); got < lo || got > d {
			t.Fatalf("attempt %d max-jitter: %v not in [%v, %v]", attempt, got, lo, d)
		}
	}
}

func TestRetryHonorsRetryAfterOverJitter(t *testing.T) {
	inner := &scriptedSender{results: []Result{
		{StatusCode: 503, RetryAfter: 2 * time.Second},
		{Delivered: true, StatusCode: 200},
	}}
	var waits []time.Duration
	rs := NewRetryingSender(inner, stubRetryCfg{attempts: 3, base: 100 * time.Millisecond, max: time.Minute})
	rs.sleep = func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	rs.jitter = func(int64) int64 { return 0 } // minimal backoff so Retry-After clearly dominates
	if _, err := rs.Send(context.Background(), "t", "apns", "ref", SendOptions{}, Message{}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(waits) != 1 {
		t.Fatalf("want 1 sleep, got %d", len(waits))
	}
	if waits[0] != 2*time.Second {
		t.Fatalf("want Retry-After (2s) to win over jittered backoff, got %v", waits[0])
	}
}

func TestRetryExhaustsAttempts(t *testing.T) {
	inner := &scriptedSender{errs: []error{errors.New("x"), errors.New("x"), errors.New("x")}}
	r := newNoSleepRetrier(inner, 3)
	_, err := r.Send(context.Background(), "t", "apns", "ref", SendOptions{}, Message{})
	if err == nil || inner.calls != 3 {
		t.Fatalf("expected 3 attempts then error, got calls=%d err=%v", inner.calls, err)
	}
}
