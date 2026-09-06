package api

import (
	"context"
	"net/http"
	"time"

	"github.com/MunifTanjim/pushport/internal/ratelimit"
)

type AuthConfigProvider interface {
	AuthFailConfig(ctx context.Context) (perMin, burst int)
}

type AuthConfigFunc func(ctx context.Context) (perMin, burst int)

func (f AuthConfigFunc) AuthFailConfig(ctx context.Context) (int, int) { return f(ctx) }

// AuthThrottle blocks a client IP that has produced too many recent
// authentication failures (HTTP 401), defending the static admin token and any
// other bearer credential against online brute-forcing. It counts failures in a
// per-IP token bucket: a burst of failures is tolerated, then the IP is blocked
// with 429 until the bucket refills. Successful requests cost nothing, so
// legitimate callers are never throttled.
type AuthThrottle struct {
	limiter *ratelimit.Limiter
	cfg     AuthConfigProvider
}

func NewAuthThrottle(limiter *ratelimit.Limiter, cfg AuthConfigProvider) *AuthThrottle {
	return &AuthThrottle{limiter: limiter, cfg: cfg}
}

// Wrap returns next guarded by the per-IP auth-failure limit. A blocked IP is
// rejected before next runs; otherwise next runs and a failure token is
// consumed only when it responds 401.
//
// Two deliberate design choices (not oversights):
//
//   - Only 401 is counted, not 403. The online-brute-force vectors — a bad admin,
//     pat_, pit_, or device token — all return 401, so credential guessing is
//     throttled. 403 means authenticated-but-forbidden and includes legitimate
//     business-rule denials (a failed Turnstile on the public registration page,
//     self-registration disabled, deleting the undeletable default plan); counting
//     those would throttle real users by IP. The final status can't distinguish a
//     cross-tenant probe from a legit 403, so 401-only is the precise signal.
//
//   - The gate peeks (Allowed) without consuming, and consumes (Allow) only after
//     a 401, so a successful request costs nothing and legitimate callers — even
//     highly concurrent ones — are never throttled. This leaves a small TOCTOU
//     window where concurrent failures can slip past the peek before any consume
//     lands, briefly exceeding burst. Closing it would require holding a token
//     across the handler (reserve-then-refund), which would throttle concurrent
//     *successful* traffic — a worse regression. The limiter still bounds the
//     sustained failure rate, which is what matters against credential guessing.
func (t *AuthThrottle) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		perMin, burst := t.cfg.AuthFailConfig(r.Context())
		key := "authfail:" + clientIP(r)
		if !t.limiter.Allowed(key, perMin, burst) {
			e := ErrorTooManyRequests().WithMessage("too many failed attempts")
			if perMin > 0 {
				// Time for one failure token to refill.
				e.WithRetryAfter(retryAfterSeconds(time.Minute / time.Duration(perMin)))
			}
			SendError(w, r, e)
			return
		}
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sr, r)
		if sr.status == http.StatusUnauthorized {
			t.limiter.Allow(key, perMin, burst)
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.wroteHeader = true // an implicit 200
	}
	return s.ResponseWriter.Write(b)
}
