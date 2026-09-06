package settings

import (
	"context"
	"sync"
	"time"

	"github.com/MunifTanjim/pushport/internal/db"
)

type ServerSettings struct {
	RetryMaxAttempts int
	RetryBaseBackoff time.Duration
	RetryMaxBackoff  time.Duration
	AuthFailIPPerMin int
	AuthFailIPBurst  int
}

var Defaults = ServerSettings{
	RetryMaxAttempts: 3,
	RetryBaseBackoff: 200 * time.Millisecond,
	RetryMaxBackoff:  5000 * time.Millisecond,
	AuthFailIPPerMin: 5,
	AuthFailIPBurst:  5,
}

// Querier is the slice of the DB API the Reader needs (narrowed for stubbing).
type Querier interface {
	GetServerSetting(ctx context.Context) (db.ServerSetting, error)
}

// Reader is a cached accessor for the single settings row: a successful read is
// cached until Flush; a read error fails open to Defaults and is not cached.
type Reader struct {
	q Querier

	mu     sync.Mutex
	cached *ServerSettings
}

func NewReader(q Querier) *Reader { return &Reader{q: q} }

func (r *Reader) Get(ctx context.Context) ServerSettings {
	r.mu.Lock()
	if r.cached != nil {
		s := *r.cached
		r.mu.Unlock()
		return s
	}
	r.mu.Unlock()

	row, err := r.q.GetServerSetting(ctx)
	if err != nil {
		return Defaults
	}
	s := ServerSettings{
		RetryMaxAttempts: int(row.RetryMaxAttempts),
		RetryBaseBackoff: parseDurationOr(row.RetryBaseBackoff, Defaults.RetryBaseBackoff),
		RetryMaxBackoff:  parseDurationOr(row.RetryMaxBackoff, Defaults.RetryMaxBackoff),
		AuthFailIPPerMin: int(row.AuthFailIpPerMin),
		AuthFailIPBurst:  int(row.AuthFailIpBurst),
	}
	r.mu.Lock()
	r.cached = &s
	r.mu.Unlock()
	return s
}

func parseDurationOr(s string, def time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

// Flush drops the cache. Call after any settings update so the next Get re-reads.
func (r *Reader) Flush() {
	r.mu.Lock()
	r.cached = nil
	r.mu.Unlock()
}

// RetryConfig adapts Get for transport.RetryConfigProvider.
func (r *Reader) RetryConfig(ctx context.Context) (attempts int, base, max time.Duration) {
	s := r.Get(ctx)
	return s.RetryMaxAttempts, s.RetryBaseBackoff, s.RetryMaxBackoff
}

// AuthFailConfig adapts Get for the API auth throttle (per client IP).
func (r *Reader) AuthFailConfig(ctx context.Context) (perMin, burst int) {
	s := r.Get(ctx)
	return s.AuthFailIPPerMin, s.AuthFailIPBurst
}
