package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MunifTanjim/pushport/internal/ratelimit"
)

type stubAuthCfg struct{ perMin, burst int }

func (s stubAuthCfg) AuthFailConfig(context.Context) (int, int) { return s.perMin, s.burst }

func TestAuthThrottle(t *testing.T) {
	now := time.Unix(1000, 0)
	lim := ratelimit.NewLimiter(func() time.Time { return now })
	th := NewAuthThrottle(lim, stubAuthCfg{60, 2})

	unauthorized := th.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SendError(w, r, ErrorUnauthorized())
	}))
	call := func(h http.Handler, ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/apps", nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("blocks an IP after burst of auth failures", func(t *testing.T) {
		if c := call(unauthorized, "1.1.1.1").Code; c != http.StatusUnauthorized {
			t.Fatalf("1st want 401, got %d", c)
		}
		if c := call(unauthorized, "1.1.1.1").Code; c != http.StatusUnauthorized {
			t.Fatalf("2nd want 401, got %d", c)
		}
		rec := call(unauthorized, "1.1.1.1")
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("3rd want 429, got %d", rec.Code)
		}
		if rec.Header().Get("Retry-After") == "" {
			t.Fatal("429 should carry a Retry-After header")
		}
	})

	t.Run("failures are isolated per IP", func(t *testing.T) {
		if c := call(unauthorized, "2.2.2.2").Code; c != http.StatusUnauthorized {
			t.Fatalf("fresh IP want 401, got %d", c)
		}
	})

	t.Run("successful requests never accrue a block", func(t *testing.T) {
		ok := th.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			SendData(w, r, http.StatusOK, "ok")
		}))
		for i := 0; i < 10; i++ {
			if c := call(ok, "3.3.3.3").Code; c != http.StatusOK {
				t.Fatalf("success #%d want 200, got %d", i, c)
			}
		}
	})
}

func TestAuthThrottleReadsProvider(t *testing.T) {
	now := time.Unix(1000, 0)
	lim := ratelimit.NewLimiter(func() time.Time { return now })
	th := NewAuthThrottle(lim, stubAuthCfg{perMin: 60, burst: 2})

	unauthorized := th.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SendError(w, r, ErrorUnauthorized())
	}))
	call := func(h http.Handler, ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/apps", nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("blocks an IP after burst of auth failures", func(t *testing.T) {
		if c := call(unauthorized, "4.4.4.4").Code; c != http.StatusUnauthorized {
			t.Fatalf("1st want 401, got %d", c)
		}
		if c := call(unauthorized, "4.4.4.4").Code; c != http.StatusUnauthorized {
			t.Fatalf("2nd want 401, got %d", c)
		}
		rec := call(unauthorized, "4.4.4.4")
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("3rd want 429, got %d", rec.Code)
		}
		if rec.Header().Get("Retry-After") == "" {
			t.Fatal("429 should carry a Retry-After header")
		}
	})

	t.Run("failures are isolated per IP", func(t *testing.T) {
		if c := call(unauthorized, "5.5.5.5").Code; c != http.StatusUnauthorized {
			t.Fatalf("fresh IP want 401, got %d", c)
		}
	})

	t.Run("successful requests never accrue a block", func(t *testing.T) {
		ok := th.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			SendData(w, r, http.StatusOK, "ok")
		}))
		for i := 0; i < 10; i++ {
			if c := call(ok, "6.6.6.6").Code; c != http.StatusOK {
				t.Fatalf("success #%d want 200, got %d", i, c)
			}
		}
	})
}
