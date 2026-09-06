package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type entry struct {
	lim      *rate.Limiter
	perMin   int
	burst    int
	lastSeen time.Time
}

type Limiter struct {
	now func() time.Time

	mu sync.Mutex
	m  map[string]*entry
}

func NewLimiter(now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{now: now, m: make(map[string]*entry)}
}

// bucketAt returns key's bucket, creating or resyncing it to (perMin, burst) in
// place. Caller must hold perMin > 0.
func (l *Limiter) bucketAt(now time.Time, key string, perMin, burst int) *rate.Limiter {
	if burst < 1 {
		burst = 1
	}
	limit := rate.Limit(float64(perMin) / 60.0) // tokens per second
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.m[key]
	if !ok {
		e = &entry{lim: rate.NewLimiter(limit, burst), perMin: perMin, burst: burst}
		l.m[key] = e
	} else if e.perMin != perMin || e.burst != burst {
		e.lim.SetLimitAt(now, limit)
		e.lim.SetBurstAt(now, burst)
		e.perMin, e.burst = perMin, burst
	}
	e.lastSeen = now
	return e.lim
}

// Allow consumes one token for key. perMin <= 0 always allows and creates no
// bucket; a changed (perMin, burst) resyncs the bucket in place. When denied,
// retryAfter is the wait until a token would be available.
func (l *Limiter) Allow(key string, perMin, burst int) (bool, time.Duration) {
	if perMin <= 0 {
		return true, 0
	}
	now := l.now()
	lim := l.bucketAt(now, key, perMin, burst)
	r := lim.ReserveN(now, 1)
	if !r.OK() {
		return false, 0
	}
	if d := r.DelayFrom(now); d > 0 {
		r.CancelAt(now) // reject with the delay; CancelAt keeps the bucket clock consistent
		return false, d
	}
	return true, 0
}

// Reserve consumes one token for key like Allow, but defers the keep/cancel
// decision to the caller: on success it returns a non-nil cancel func that
// restores the token (best-effort, always conservative). perMin <= 0 always
// allows with a no-op cancel; a denial returns ok=false with retryAfter and
// holds nothing.
func (l *Limiter) Reserve(key string, perMin, burst int) (cancel func(), ok bool, retry time.Duration) {
	if perMin <= 0 {
		return func() {}, true, 0
	}
	now := l.now()
	lim := l.bucketAt(now, key, perMin, burst)
	r := lim.ReserveN(now, 1)
	if !r.OK() {
		return nil, false, 0
	}
	if d := r.DelayFrom(now); d > 0 {
		r.CancelAt(now)
		return nil, false, d
	}
	// Cancel at the reservation's own time: rate.Reservation.CancelAt refuses to
	// restore a token once its timeToAct is before the passed time, so a later
	// l.now() would silently drop the refund.
	return func() { r.CancelAt(now) }, true, 0
}

// Allowed reports whether key currently has a token available, without
// consuming one. perMin <= 0 is always allowed. An unseen key has a full
// bucket and is allowed without creating an entry (so peeks alone never
// populate the map).
func (l *Limiter) Allowed(key string, perMin, burst int) bool {
	if perMin <= 0 {
		return true
	}
	now := l.now()
	l.mu.Lock()
	e, ok := l.m[key]
	l.mu.Unlock()
	if !ok {
		return true
	}
	return e.lim.TokensAt(now) >= 1
}

// Cleanup evicts buckets not accessed within idle, bounding memory.
func (l *Limiter) Cleanup(idle time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, e := range l.m {
		if now.Sub(e.lastSeen) > idle {
			delete(l.m, k)
		}
	}
}

func (l *Limiter) numKeys() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.m)
}
