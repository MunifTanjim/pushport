package api

import (
	"context"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MunifTanjim/pushport/internal/app"
	"github.com/MunifTanjim/pushport/internal/crypto"
	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/ratelimit"
	"github.com/MunifTanjim/pushport/internal/seal"
)

func nil_ctx() context.Context { return context.Background() }

func newDeviceServer(t *testing.T) (*httptest.Server, *app.Service, *seal.Service) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	appSvc := app.NewService(d, kr)
	sealSvc := seal.NewService(appSvc)
	h := NewDeviceHandler(sealSvc, appSvc, "http://relay.test", 24*time.Hour, time.Minute, 48*time.Hour)
	mux := http.NewServeMux()
	h.Routes(mux)
	srv := httptest.NewServer(WithRequestID(mux))
	t.Cleanup(srv.Close)
	return srv, appSvc, sealSvc
}

func TestSubscribeMintsRoundTrippableEndpoint(t *testing.T) {
	srv, appSvc, sealSvc := newDeviceServer(t)
	a, _ := appSvc.Create(nil_ctx(), "app")

	resp, err := http.Post(srv.URL+"/apps/"+a.ID+"/subscribe", "application/json",
		strings.NewReader(`{"transport":"apns","token":"devtoken"}`))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: err=%v status=%d", err, resp.StatusCode)
	}
	var out struct {
		Endpoint  string `json:"endpoint"`
		ExpiresAt int64  `json:"expires_at"`
	}
	dataOf(t, resp, &out)
	if !strings.HasPrefix(out.Endpoint, "http://relay.test/push/") {
		t.Fatalf("bad endpoint: %s", out.Endpoint)
	}
	token := strings.TrimPrefix(out.Endpoint, "http://relay.test/push/")
	gotApp, p, err := sealSvc.Open(nil_ctx(), token)
	if err != nil || gotApp != a.ID || p.Transport != "apns" || p.TransportRef != "devtoken" {
		t.Fatalf("open minted token: app=%s payload=%+v err=%v", gotApp, p, err)
	}
	if p.Sandbox {
		t.Fatal("default subscribe should not set sandbox")
	}
}

func TestSubscribeSandboxToggle(t *testing.T) {
	srv, appSvc, sealSvc := newDeviceServer(t)
	a, _ := appSvc.Create(nil_ctx(), "app")

	post := func(body string) seal.Payload {
		t.Helper()
		resp, err := http.Post(srv.URL+"/apps/"+a.ID+"/subscribe", "application/json", strings.NewReader(body))
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("subscribe %s: err=%v status=%d", body, err, resp.StatusCode)
		}
		var out struct {
			Endpoint string `json:"endpoint"`
		}
		dataOf(t, resp, &out)
		_, p, err := sealSvc.Open(nil_ctx(), strings.TrimPrefix(out.Endpoint, "http://relay.test/push/"))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		return p
	}

	if p := post(`{"transport":"apns","token":"t1","sandbox":true}`); !p.Sandbox {
		t.Fatal("sandbox=true should be sealed into the endpoint")
	}
	// sandbox is APNs-only; it must not leak into other transports
	if p := post(`{"transport":"fcm","token":"t2","sandbox":true}`); p.Sandbox {
		t.Fatal("sandbox must be ignored for non-apns transports")
	}
	if p := post(`{"transport":"webpush","token":"t3","sandbox":true}`); p.Sandbox {
		t.Fatal("sandbox must be ignored for non-apns transports")
	}
}

func TestSubscribeUnknownApp404(t *testing.T) {
	srv, _, _ := newDeviceServer(t)
	resp, _ := http.Post(srv.URL+"/apps/nope/subscribe", "application/json",
		strings.NewReader(`{"transport":"apns","token":"x"}`))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestSubscribeTTL(t *testing.T) {
	srv, appSvc, _ := newDeviceServer(t) // default 24h, min 1m, max 48h
	a, _ := appSvc.Create(nil_ctx(), "app")

	post := func(bodyJSON string) (*http.Response, int64) {
		t.Helper()
		before := time.Now().Unix()
		resp, err := http.Post(srv.URL+"/apps/"+a.ID+"/subscribe", "application/json", strings.NewReader(bodyJSON))
		if err != nil {
			t.Fatalf("register: %v", err)
		}
		return resp, before
	}
	delta := func(resp *http.Response, before int64) int64 {
		t.Helper()
		var out struct {
			ExpiresAt int64 `json:"expires_at"`
		}
		dataOf(t, resp, &out)
		return out.ExpiresAt - before
	}

	if resp, before := post(`{"transport":"apns","token":"t","ttl":"1h"}`); resp.StatusCode != http.StatusCreated {
		t.Fatalf("in-range ttl: status=%d", resp.StatusCode)
	} else if d := delta(resp, before); d < 3595 || d > 3605 {
		t.Fatalf("in-range ttl not honored: got ~%ds, want ~3600", d)
	}

	if resp, before := post(`{"transport":"apns","token":"t"}`); resp.StatusCode != http.StatusCreated {
		t.Fatalf("default ttl: status=%d", resp.StatusCode)
	} else if d := delta(resp, before); d < 24*3600-5 || d > 24*3600+5 {
		t.Fatalf("default ttl wrong: got ~%ds, want ~%d", d, 24*3600)
	}

	for _, tc := range []struct{ name, body string }{
		{"over-max", `{"transport":"apns","token":"t","ttl":"100h"}`},
		{"under-min", `{"transport":"apns","token":"t","ttl":"30s"}`},
		{"malformed", `{"transport":"apns","token":"t","ttl":"nope"}`},
	} {
		resp, _ := post(tc.body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s ttl: want 400, got %d", tc.name, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func newRateLimitedDeviceServer(t *testing.T, subscribePerMin, subscribeBurst int64) (*httptest.Server, *app.Service, *ratelimit.Resolver, *db.DB) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	appSvc := app.NewService(d, kr)
	sealSvc := seal.NewService(appSvc)
	h := NewDeviceHandler(sealSvc, appSvc, "http://relay.test", 24*time.Hour, time.Minute, 48*time.Hour)
	lim := ratelimit.NewLimiter(func() time.Time { return time.Unix(1000, 0) }) // frozen clock, no refill
	resolver := ratelimit.NewResolver(d.Queries)
	h.SetRateLimit(lim, resolver)
	mux := http.NewServeMux()
	h.Routes(mux)
	srv := httptest.NewServer(WithRequestID(mux))
	t.Cleanup(srv.Close)
	return srv, appSvc, resolver, d
}

// applySubscribePlan updates the global default app plan (created by appSvc.Create)
// to the given per-min / burst values and flushes the resolver cache.
func applySubscribePlan(t *testing.T, d *db.DB, resolver *ratelimit.Resolver, perMin, burst int64) {
	t.Helper()
	ctx := nil_ctx()
	plan, err := d.Queries.GetDefaultAppUsagePlan(ctx)
	if err != nil {
		t.Fatalf("GetDefaultAppUsagePlan: %v", err)
	}
	if _, err := d.Queries.UpdateAppUsagePlan(ctx, db.UpdateAppUsagePlanParams{
		ID:              plan.ID,
		SubscribePerMin: db.NewNullInt64(perMin),
		SubscribeBurst:  db.NewNullInt64(burst),
	}); err != nil {
		t.Fatalf("UpdateAppUsagePlan: %v", err)
	}
	resolver.Flush()
}

func TestSubscribePerIPRateLimit(t *testing.T) {
	srv, appSvc, resolver, d := newRateLimitedDeviceServer(t, 0, 0)

	a, _ := appSvc.Create(nil_ctx(), "app")
	applySubscribePlan(t, d, resolver, 60, 2)

	post := func() int {
		resp, err := http.Post(srv.URL+"/apps/"+a.ID+"/subscribe", "application/json",
			strings.NewReader(`{"transport":"apns","token":"devtoken"}`))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	if c := post(); c != http.StatusCreated {
		t.Fatalf("1st want 201, got %d", c)
	}
	if c := post(); c != http.StatusCreated {
		t.Fatalf("2nd want 201, got %d", c)
	}
	if c := post(); c != http.StatusTooManyRequests {
		t.Fatalf("3rd want 429, got %d", c)
	}
}

func TestSubscribePerAppKeying(t *testing.T) {
	// Proves per-app keying: app B's bucket is independent of app A's.
	srv, appSvc, resolver, d := newRateLimitedDeviceServer(t, 0, 0)

	appA, _ := appSvc.Create(nil_ctx(), "app-a")
	appB, _ := appSvc.Create(nil_ctx(), "app-b")
	applySubscribePlan(t, d, resolver, 1, 1) // burst=1: one token per app, no refill (frozen clock)

	postTo := func(appID string) int {
		resp, err := http.Post(srv.URL+"/apps/"+appID+"/subscribe", "application/json",
			strings.NewReader(`{"transport":"apns","token":"devtoken"}`))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	if c := postTo(appA.ID); c != http.StatusCreated {
		t.Fatalf("app-a 1st: want 201, got %d", c)
	}
	if c := postTo(appA.ID); c != http.StatusTooManyRequests {
		t.Fatalf("app-a 2nd: want 429, got %d", c)
	}
	if c := postTo(appB.ID); c != http.StatusCreated {
		t.Fatalf("app-b 1st: want 201, got %d (per-app keying broken: A's bucket bled into B)", c)
	}
}

func TestSubscribeBadTransport400(t *testing.T) {
	srv, appSvc, _ := newDeviceServer(t)
	a, _ := appSvc.Create(nil_ctx(), "app")
	resp, _ := http.Post(srv.URL+"/apps/"+a.ID+"/subscribe", "application/json",
		strings.NewReader(`{"transport":"carrier-pigeon","token":"x"}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}
