package api

import (
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MunifTanjim/pushport/internal/app"
	"github.com/MunifTanjim/pushport/internal/crypto"
	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/instance"
	"github.com/MunifTanjim/pushport/internal/ratelimit"
)

// newSettingsServer builds a server like newAdminServer but wires an
// OnSettingsChange hook and returns the fired flag.
func newSettingsServer(t *testing.T) (*httptest.Server, *bool) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	apps := app.NewService(d, kr)
	instances := instance.NewService(d.Queries, kr)
	h := NewManagementHandler(apps, instances, d.Queries, "admintok")
	fired := false
	h.OnSettingsChange(func() { fired = true })
	mux := http.NewServeMux()
	h.Routes(mux)
	NewInstanceRegistrationHandler(instances, apps, ratelimit.NewLimiter(nil), ratelimit.NewResolver(d.Queries), "admintok", &stubVerifier{ok: true}).Routes(mux)
	srv := httptest.NewServer(WithRequestID(mux))
	t.Cleanup(srv.Close)
	return srv, &fired
}

func TestServerSettingsRejectsInvalid(t *testing.T) {
	srv, _ := newSettingsServer(t)
	c := srv.Client()
	cases := []string{
		`{"auth_fail_ip_per_min":-1}`,
		`{"auth_fail_ip_burst":-1}`,
		`{"retry_max_attempts":0}`,
		`{"retry_base_backoff":"0s"}`,
		`{"retry_base_backoff":"-5s"}`,
		`{"retry_base_backoff":"notaduration"}`,
	}
	for _, body := range cases {
		resp, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/settings", "Bearer admintok", body))
		if err != nil || resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("PATCH %s: err=%v status=%d, want 400", body, err, resp.StatusCode)
		}
	}
}

func TestServerSettingsGetAndPatch(t *testing.T) {
	srv, hookFired := newSettingsServer(t)
	c := srv.Client()

	resp, err := c.Do(jsonReq(t, "GET", srv.URL+"/settings", "Bearer admintok", ""))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET settings: err=%v status=%d", err, resp.StatusCode)
	}
	var got struct {
		RetryMaxAttempts int    `json:"retry_max_attempts"`
		RetryBaseBackoff string `json:"retry_base_backoff"`
		RetryMaxBackoff  string `json:"retry_max_backoff"`
		AuthFailIPPerMin int    `json:"auth_fail_ip_per_min"`
		AuthFailIPBurst  int    `json:"auth_fail_ip_burst"`
	}
	dataOf(t, resp, &got)
	if got.RetryMaxAttempts != 3 {
		t.Errorf("default retry_max_attempts: want 3, got %d", got.RetryMaxAttempts)
	}
	if got.AuthFailIPPerMin != 5 {
		t.Errorf("default auth_fail_ip_per_min: want 5, got %d", got.AuthFailIPPerMin)
	}

	if *hookFired {
		t.Fatal("hook should not have fired before PATCH")
	}

	resp2, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/settings", "Bearer admintok", `{"retry_max_attempts":9}`))
	if err != nil || resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("PATCH settings: err=%v status=%d", err, resp2.StatusCode)
	}
	if !*hookFired {
		t.Fatal("OnSettingsChange hook should have fired after PATCH")
	}

	resp3, err := c.Do(jsonReq(t, "GET", srv.URL+"/settings", "Bearer admintok", ""))
	if err != nil || resp3.StatusCode != http.StatusOK {
		t.Fatalf("GET settings after patch: err=%v status=%d", err, resp3.StatusCode)
	}
	var got2 struct {
		RetryMaxAttempts int `json:"retry_max_attempts"`
		AuthFailIPPerMin int `json:"auth_fail_ip_per_min"`
	}
	dataOf(t, resp3, &got2)
	if got2.RetryMaxAttempts != 9 {
		t.Errorf("after patch: want retry_max_attempts=9, got %d", got2.RetryMaxAttempts)
	}
	if got2.AuthFailIPPerMin != 5 {
		t.Errorf("after patch: auth_fail_ip_per_min should remain 5, got %d", got2.AuthFailIPPerMin)
	}

	resp4, err := c.Do(jsonReq(t, "GET", srv.URL+"/settings", "Bearer wrongtoken", ""))
	if err != nil || resp4.StatusCode != http.StatusUnauthorized {
		t.Fatalf("non-admin GET settings: err=%v status=%d", err, resp4.StatusCode)
	}
}
