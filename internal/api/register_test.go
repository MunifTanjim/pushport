package api

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MunifTanjim/pushport/internal/app"
	"github.com/MunifTanjim/pushport/internal/crypto"
	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/instance"
	"github.com/MunifTanjim/pushport/internal/ratelimit"
)

type stubVerifier struct {
	ok        bool
	err       error
	gotSecret string
}

func (s *stubVerifier) Verify(_ context.Context, secret, _, _ string) (bool, error) {
	s.gotSecret = secret
	return s.ok, s.err
}

// newRegServerT is like newRegServer but with an explicit verifier.
func newRegServerT(t *testing.T, v *stubVerifier) (*httptest.Server, *app.Service) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	apps := app.NewService(d, kr)
	instances := instance.NewService(d.Queries, kr)
	resolver := ratelimit.NewResolver(d.Queries)
	mux := http.NewServeMux()
	NewInstanceRegistrationHandler(instances, apps, ratelimit.NewLimiter(nil), resolver, "admintok", v).Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, apps
}

func TestPublicCreateTurnstile(t *testing.T) {
	v := &stubVerifier{ok: true}
	srv, apps := newRegServerT(t, v)
	id := mkApp(t, apps, true)
	if err := apps.SetTurnstile(t.Context(), id, "app-site", "app-secret"); err != nil {
		t.Fatalf("set turnstile: %v", err)
	}

	if s := regPost(t, srv, "/apps/"+id+"/instances", "", `{}`); s != http.StatusForbidden {
		t.Fatalf("missing token: want 403 got %d", s)
	}
	if s := regPost(t, srv, "/apps/"+id+"/instances", "", `{"label":"x","turnstile_token":"tok"}`); s != http.StatusCreated {
		t.Fatalf("valid token: want 201 got %d", s)
	}
	if v.gotSecret != "app-secret" {
		t.Fatalf("expected app secret, got %q", v.gotSecret)
	}
}

func TestPublicCreateTurnstileInvalid(t *testing.T) {
	v := &stubVerifier{ok: false}
	srv, apps := newRegServerT(t, v)
	id := mkApp(t, apps, true)
	if err := apps.SetTurnstile(t.Context(), id, "app-site", "app-secret"); err != nil {
		t.Fatalf("set turnstile: %v", err)
	}
	if s := regPost(t, srv, "/apps/"+id+"/instances", "", `{"turnstile_token":"tok"}`); s != http.StatusForbidden {
		t.Fatalf("invalid token: want 403 got %d", s)
	}
}

// newRegServer builds a server mounting only the registration handler.
// Apps are created directly via app.Service (not through an HTTP handler).
func newRegServer(t *testing.T) (*httptest.Server, *app.Service) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	apps := app.NewService(d, kr)
	instances := instance.NewService(d.Queries, kr)
	limiter := ratelimit.NewLimiter(nil)
	resolver := ratelimit.NewResolver(d.Queries)
	mux := http.NewServeMux()
	NewInstanceRegistrationHandler(instances, apps, limiter, resolver, "admintok", &stubVerifier{}).Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, apps
}

func mkApp(t *testing.T, apps *app.Service, public bool) string {
	t.Helper()
	a, err := apps.Create(t.Context(), "app")
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	if public {
		if err := apps.SetPublic(t.Context(), a.ID, true); err != nil {
			t.Fatalf("set public: %v", err)
		}
	}
	return a.ID
}

func regPost(t *testing.T, srv *httptest.Server, path, auth, body string) int {
	t.Helper()
	req, _ := http.NewRequest("POST", srv.URL+path, strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

func getPage(t *testing.T, srv *httptest.Server, path string) (int, string) {
	t.Helper()
	resp, err := srv.Client().Get(srv.URL + path)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestRegisterPage(t *testing.T) {
	v := &stubVerifier{ok: true}
	srv, apps := newRegServerT(t, v)
	pubID := mkApp(t, apps, true)
	privID := mkApp(t, apps, false)
	if err := apps.SetTurnstile(t.Context(), pubID, "app-site", "app-secret"); err != nil {
		t.Fatalf("set turnstile: %v", err)
	}

	code, body := getPage(t, srv, "/apps/"+pubID+"/instances/register")
	if code != http.StatusOK {
		t.Fatalf("page: want 200 got %d", code)
	}
	if !strings.Contains(body, "app-site") {
		t.Fatalf("page missing site key: %s", body)
	}

	// A private app and an unknown app must be indistinguishable, so the page
	// can't be probed to learn which private apps exist.
	privCode, privBody := getPage(t, srv, "/apps/"+privID+"/instances/register")
	bogusCode, bogusBody := getPage(t, srv, "/apps/bogus/instances/register")
	if privCode != http.StatusNotFound {
		t.Fatalf("private page: want 404 got %d", privCode)
	}
	if bogusCode != http.StatusNotFound {
		t.Fatalf("unknown page: want 404 got %d", bogusCode)
	}
	if privBody != bogusBody {
		t.Fatalf("private and unknown responses differ (existence oracle):\n private: %s\n unknown: %s", privBody, bogusBody)
	}
}

func TestRegisterPageWithoutTurnstile(t *testing.T) {
	srv, apps := newRegServerT(t, &stubVerifier{})
	pubID := mkApp(t, apps, true)
	// Turnstile is optional: an unconfigured public app still serves the form,
	// just without a challenge widget.
	code, body := getPage(t, srv, "/apps/"+pubID+"/instances/register")
	if code != http.StatusOK {
		t.Fatalf("page: want 200 got %d", code)
	}
	if strings.Contains(body, "data-sitekey") {
		t.Fatalf("page should omit the turnstile widget when unconfigured: %s", body)
	}
}

func TestCreateInstanceOwnerMode(t *testing.T) {
	srv, apps := newRegServer(t)
	id := mkApp(t, apps, false) // private app; owner can still create
	appToken, err := apps.GenerateAppToken(t.Context(), id)
	if err != nil {
		t.Fatalf("app token: %v", err)
	}

	if s := regPost(t, srv, "/apps/"+id+"/instances", "Bearer admintok", `{"label":"a"}`); s != http.StatusCreated {
		t.Fatalf("admin create: %d", s)
	}
	if s := regPost(t, srv, "/apps/"+id+"/instances", "Bearer "+appToken, `{"label":"b"}`); s != http.StatusCreated {
		t.Fatalf("app create: %d", s)
	}
	if s := regPost(t, srv, "/apps/"+id+"/instances", "Bearer pat_wrong", `{}`); s != http.StatusUnauthorized {
		t.Fatalf("bad token: want 401 got %d", s)
	}
	if s := regPost(t, srv, "/apps/bogus/instances", "Bearer admintok", `{}`); s != http.StatusNotFound {
		t.Fatalf("unknown app: want 404 got %d", s)
	}
}

func TestCreateInstancePublicMode(t *testing.T) {
	srv, apps := newRegServer(t)
	pubID := mkApp(t, apps, true)
	privID := mkApp(t, apps, false)

	if s := regPost(t, srv, "/apps/"+pubID+"/instances", "", `{"label":"x"}`); s != http.StatusCreated {
		t.Fatalf("public mint: %d", s)
	}
	if s := regPost(t, srv, "/apps/"+privID+"/instances", "", `{}`); s != http.StatusForbidden {
		t.Fatalf("private self-reg: want 403 got %d", s)
	}
	if s := regPost(t, srv, "/apps/bogus/instances", "", `{}`); s != http.StatusNotFound {
		t.Fatalf("unknown app: want 404 got %d", s)
	}
}

func TestCreateInstanceCrossAppToken(t *testing.T) {
	srv, apps := newRegServer(t)
	idA := mkApp(t, apps, false)
	idB := mkApp(t, apps, false)
	tokenB, err := apps.GenerateAppToken(t.Context(), idB)
	if err != nil {
		t.Fatalf("app token: %v", err)
	}
	if s := regPost(t, srv, "/apps/"+idA+"/instances", "Bearer "+tokenB, `{}`); s != http.StatusUnauthorized {
		t.Fatalf("cross-app token: want 401 got %d", s)
	}
}

func TestCreateInstanceRateLimit(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	appsvc := app.NewService(d, kr)
	instances := instance.NewService(d.Queries, kr)
	if err := d.Queries.CreateDefaultAppUsagePlan(t.Context(), db.CreateDefaultAppUsagePlanParams{
		ID:             "default-app-plan",
		Name:           "default",
		RegisterPerMin: 1,
		RegisterBurst:  1,
		CreatedAt:      1,
	}); err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	resolver := ratelimit.NewResolver(d.Queries)
	mux := http.NewServeMux()
	NewInstanceRegistrationHandler(instances, appsvc, ratelimit.NewLimiter(nil), resolver, "admintok", &stubVerifier{ok: true}).Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	id := mkApp(t, appsvc, true)
	if s := regPost(t, srv, "/apps/"+id+"/instances", "", `{"label":"x"}`); s != http.StatusCreated {
		t.Fatalf("first request: want 201 got %d", s)
	}
	got429 := false
	for range 10 {
		if s := regPost(t, srv, "/apps/"+id+"/instances", "", `{"label":"x"}`); s == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatal("rate-limit: want 429 but never received it")
	}
}

func TestCreateInstancePerIPRateLimit(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	appsvc := app.NewService(d, kr)
	instances := instance.NewService(d.Queries, kr)
	if err := d.Queries.CreateDefaultAppUsagePlan(t.Context(), db.CreateDefaultAppUsagePlanParams{
		ID:               "default-app-plan",
		Name:             "default",
		RegisterPerMin:   0,
		RegisterBurst:    0,
		RegisterIpPerMin: 60,
		RegisterIpBurst:  2,
		CreatedAt:        1,
	}); err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	resolver := ratelimit.NewResolver(d.Queries)
	// App-global limit is unlimited (0); the per-IP floor is what should bite.
	h := NewInstanceRegistrationHandler(instances, appsvc, ratelimit.NewLimiter(nil), resolver, "admintok", &stubVerifier{ok: true})
	ipLim := ratelimit.NewLimiter(func() time.Time { return time.Unix(1000, 0) })
	h.SetIPRateLimit(ipLim)
	mux := http.NewServeMux()
	h.Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	id := mkApp(t, appsvc, true)
	if s := regPost(t, srv, "/apps/"+id+"/instances", "", `{"label":"x"}`); s != http.StatusCreated {
		t.Fatalf("1st want 201, got %d", s)
	}
	if s := regPost(t, srv, "/apps/"+id+"/instances", "", `{"label":"x"}`); s != http.StatusCreated {
		t.Fatalf("2nd want 201, got %d", s)
	}
	if s := regPost(t, srv, "/apps/"+id+"/instances", "", `{}`); s != http.StatusTooManyRequests {
		t.Fatalf("3rd want 429, got %d", s)
	}
}

func TestPublicCreateTurnstileVerifyError(t *testing.T) {
	srv, apps := newRegServerT(t, &stubVerifier{err: errors.New("boom")})
	id := mkApp(t, apps, true)
	if err := apps.SetTurnstile(t.Context(), id, "app-site", "app-secret"); err != nil {
		t.Fatalf("set turnstile: %v", err)
	}
	if s := regPost(t, srv, "/apps/"+id+"/instances", "", `{"turnstile_token":"tok"}`); s != http.StatusBadGateway {
		t.Fatalf("verify error: want 502 got %d", s)
	}
}

// TestGlobalLimiterNotConsumedOnEmptyTurnstileToken proves that a POST with an
// empty turnstile_token (→ 403 "turnstile required") does not drain the
// app-global self-reg bucket: a subsequent valid request still succeeds.
func TestGlobalLimiterNotConsumedOnEmptyTurnstileToken(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})

	if err := d.Queries.CreateDefaultAppUsagePlan(t.Context(), db.CreateDefaultAppUsagePlanParams{
		ID:             "default-app-plan",
		Name:           "default",
		RegisterPerMin: 1,
		RegisterBurst:  1,
		CreatedAt:      1,
	}); err != nil {
		t.Fatalf("seed plan: %v", err)
	}

	appsvc := app.NewService(d, kr)
	instances := instance.NewService(d.Queries, kr)
	resolver := ratelimit.NewResolver(d.Queries)
	v := &stubVerifier{ok: true}

	mux := http.NewServeMux()
	NewInstanceRegistrationHandler(instances, appsvc, ratelimit.NewLimiter(nil), resolver, "admintok", v).Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	id := mkApp(t, appsvc, true)
	if err := appsvc.SetTurnstile(t.Context(), id, "site", "secret"); err != nil {
		t.Fatalf("set turnstile: %v", err)
	}

	for i := range 2 {
		s := regPost(t, srv, "/apps/"+id+"/instances", "", `{}`)
		if s != http.StatusForbidden {
			t.Fatalf("attempt %d: want 403 (turnstile required), got %d — global bucket consumed prematurely", i+1, s)
		}
	}

	if s := regPost(t, srv, "/apps/"+id+"/instances", "", `{"label":"x","turnstile_token":"good"}`); s != http.StatusCreated {
		t.Fatalf("valid request after failures: want 201, got %d", s)
	}
}

func TestGlobalLimiterNotConsumedOnTurnstileVerifyError(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})

	if err := d.Queries.CreateDefaultAppUsagePlan(t.Context(), db.CreateDefaultAppUsagePlanParams{
		ID:             "default-app-plan",
		Name:           "default",
		RegisterPerMin: 1,
		RegisterBurst:  1,
		CreatedAt:      1,
	}); err != nil {
		t.Fatalf("seed plan: %v", err)
	}

	appsvc := app.NewService(d, kr)
	instances := instance.NewService(d.Queries, kr)
	resolver := ratelimit.NewResolver(d.Queries)
	v := &stubVerifier{err: errors.New("upstream down")}

	mux := http.NewServeMux()
	NewInstanceRegistrationHandler(instances, appsvc, ratelimit.NewLimiter(nil), resolver, "admintok", v).Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	id := mkApp(t, appsvc, true)
	if err := appsvc.SetTurnstile(t.Context(), id, "site", "secret"); err != nil {
		t.Fatalf("set turnstile: %v", err)
	}

	for i := range 2 {
		s := regPost(t, srv, "/apps/"+id+"/instances", "", `{"turnstile_token":"tok"}`)
		if s != http.StatusBadGateway {
			t.Fatalf("attempt %d: want 502 (verify error), got %d — global bucket consumed prematurely", i+1, s)
		}
	}

	v.err = nil
	v.ok = true
	if s := regPost(t, srv, "/apps/"+id+"/instances", "", `{"label":"x","turnstile_token":"good"}`); s != http.StatusCreated {
		t.Fatalf("valid request after failures: want 201, got %d", s)
	}
}

func TestGlobalLimiterNotConsumedOnTurnstileFailure(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})

	if err := d.Queries.CreateDefaultAppUsagePlan(t.Context(), db.CreateDefaultAppUsagePlanParams{
		ID:             "default-app-plan",
		Name:           "default",
		RegisterPerMin: 1,
		RegisterBurst:  1,
		CreatedAt:      1,
	}); err != nil {
		t.Fatalf("seed plan: %v", err)
	}

	appsvc := app.NewService(d, kr)
	instances := instance.NewService(d.Queries, kr)
	resolver := ratelimit.NewResolver(d.Queries)
	v := &stubVerifier{ok: false}

	mux := http.NewServeMux()
	NewInstanceRegistrationHandler(instances, appsvc, ratelimit.NewLimiter(nil), resolver, "admintok", v).Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	id := mkApp(t, appsvc, true)
	if err := appsvc.SetTurnstile(t.Context(), id, "site", "secret"); err != nil {
		t.Fatalf("set turnstile: %v", err)
	}

	for i := range 2 {
		s := regPost(t, srv, "/apps/"+id+"/instances", "", `{"turnstile_token":"bad"}`)
		if s != http.StatusForbidden {
			t.Fatalf("attempt %d: want 403 (turnstile failed), got %d — global bucket consumed prematurely", i+1, s)
		}
	}

	v.ok = true
	if s := regPost(t, srv, "/apps/"+id+"/instances", "", `{"label":"x","turnstile_token":"good"}`); s != http.StatusCreated {
		t.Fatalf("valid request after failures: want 201, got %d", s)
	}
}
