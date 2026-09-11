package api

import (
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MunifTanjim/pushport/internal/app"
	"github.com/MunifTanjim/pushport/internal/crypto"
	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/instance"
	"github.com/MunifTanjim/pushport/internal/ratelimit"
)

// jsonReq builds a request, setting Authorization (when auth != "") and
// Content-Type: application/json (when body != "").
func jsonReq(t *testing.T, method, url, auth, body string) *http.Request {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// dataOf decodes the {"data": ...} envelope's data into dst.
func dataOf(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if err := json.Unmarshal(env.Data, dst); err != nil {
		t.Fatalf("decode data: %v", err)
	}
}

func newAdminServer(t *testing.T) (*httptest.Server, *db.DB, *app.Service, *instance.Service) {
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
	mux := http.NewServeMux()
	h.Routes(mux)
	NewInstanceRegistrationHandler(instances, apps, ratelimit.NewLimiter(nil), ratelimit.NewResolver(d.Queries), "admintok", &stubVerifier{ok: true}).Routes(mux)
	srv := httptest.NewServer(WithRequestID(mux))
	t.Cleanup(srv.Close)
	return srv, d, apps, instances
}

// createAppAdmin creates an app via the admin endpoint and returns the full response.
func createAppAdmin(t *testing.T, srv *httptest.Server, body string) (id, name, appToken string) {
	t.Helper()
	resp, err := srv.Client().Do(jsonReq(t, "POST", srv.URL+"/apps", "Bearer admintok", body))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("create app: err=%v status=%d", err, resp.StatusCode)
	}
	var a struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Token string `json:"token"`
	}
	dataOf(t, resp, &a)
	if a.ID == "" || !strings.HasPrefix(a.Token, "pat_") {
		t.Fatalf("bad app resp: %+v", a)
	}
	return a.ID, a.Name, a.Token
}

func TestAdminRequiresBearer(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	resp, _ := http.Post(srv.URL+"/apps", "application/json", strings.NewReader(`{"name":"x"}`))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestAdminCreateInstanceUnknownApp(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	resp, err := srv.Client().Do(jsonReq(t, "POST", srv.URL+"/apps/bogus-app-id/instances", "Bearer admintok", `{"label":"x"}`))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestAdminCreateAppAndInstance(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	resp, err := c.Do(jsonReq(t, "POST", srv.URL+"/apps", "Bearer admintok", `{"name":"myapp"}`))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("create app: err=%v status=%d", err, resp.StatusCode)
	}
	var a struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Token string `json:"token"`
	}
	dataOf(t, resp, &a)
	if a.ID == "" || a.Name != "myapp" || !strings.HasPrefix(a.Token, "pat_") {
		t.Fatalf("bad app resp: %+v", a)
	}

	resp2, err := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+a.ID+"/instances", "Bearer admintok", `{"label":"srv"}`))
	if err != nil || resp2.StatusCode != http.StatusCreated {
		t.Fatalf("create instance: err=%v status=%d", err, resp2.StatusCode)
	}
	var k struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	dataOf(t, resp2, &k)
	if !strings.HasPrefix(k.Token, "pit_") {
		t.Fatalf("bad instance resp: %+v", k)
	}
}

func TestAdminSetCredentials(t *testing.T) {
	srv, _, svc, _ := newAdminServer(t)
	c := srv.Client()

	id, _, _ := createAppAdmin(t, srv, `{"name":"app"}`)

	do := func(method, path, body string) int {
		resp, err := c.Do(jsonReq(t, method, srv.URL+path, "Bearer admintok", body))
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		return resp.StatusCode
	}

	apns := `{"key_p8":"PEM","key_id":"K1","team_id":"T1","topic":"com.x","production":true}`
	if s := do("PUT", "/apps/"+id+"/creds/apns", apns); s != http.StatusNoContent {
		t.Fatalf("set apns: status=%d", s)
	}
	sa := `{"type":"service_account","project_id":"proj"}`
	fcmBody, err := json.Marshal(map[string]string{"service_account_json": sa})
	if err != nil {
		t.Fatalf("marshal fcm body: %v", err)
	}
	if s := do("PUT", "/apps/"+id+"/creds/fcm", string(fcmBody)); s != http.StatusNoContent {
		t.Fatalf("set fcm: status=%d", s)
	}
	if s := do("PUT", "/apps/"+id+"/creds/fcm", `{"type":"service_account"}`); s != http.StatusBadRequest {
		t.Fatalf("set fcm without project_id: want 400, got %d", s)
	}
	if s := do("PUT", "/apps/"+id+"/creds/fcm", `not json`); s != http.StatusBadRequest {
		t.Fatalf("set fcm with bad json: want 400, got %d", s)
	}
	got, err := svc.GetCredentials(t.Context(), id)
	if err != nil {
		t.Fatalf("get creds: %v", err)
	}
	if got.APNs == nil || got.APNs.KeyID != "K1" || got.FCM == nil || got.FCM.ServiceAccountJSON != sa {
		t.Fatalf("both transports should be set: %+v", got)
	}

	if s := do("DELETE", "/apps/"+id+"/creds/apns", ""); s != http.StatusNoContent {
		t.Fatalf("delete apns: status=%d", s)
	}
	got, _ = svc.GetCredentials(t.Context(), id)
	if got.APNs != nil {
		t.Fatalf("apns should be gone: %+v", got.APNs)
	}
	if got.FCM == nil || got.FCM.ServiceAccountJSON != sa {
		t.Fatalf("fcm should remain: %+v", got.FCM)
	}

	if s := do("PUT", "/apps/bogus/creds/apns", apns); s != http.StatusNotFound {
		t.Fatalf("set unknown app: want 404, got %d", s)
	}
	if s := do("DELETE", "/apps/bogus/creds/webpush", ""); s != http.StatusNotFound {
		t.Fatalf("delete unknown app: want 404, got %d", s)
	}
}

func TestAdminUpdateApp(t *testing.T) {
	srv, d, _, _ := newAdminServer(t)
	c := srv.Client()

	id, _, _ := createAppAdmin(t, srv, `{"name":"pub"}`)

	resp2, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+id, "Bearer admintok", `{"name":"renamed","is_public":true}`))
	if err != nil || resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("update app: err=%v status=%d", err, resp2.StatusCode)
	}
	got, err := d.Queries.GetApp(t.Context(), id)
	if err != nil {
		t.Fatalf("get app: %v", err)
	}
	if got.Name != "renamed" || !got.IsPublic {
		t.Fatalf("update not applied: name=%q is_public=%v", got.Name, got.IsPublic)
	}

	resp3, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+id, "Bearer admintok", `{"is_public":false}`))
	if resp3.StatusCode != http.StatusNoContent {
		t.Fatalf("partial update: status=%d", resp3.StatusCode)
	}
	got, _ = d.Queries.GetApp(t.Context(), id)
	if got.Name != "renamed" || got.IsPublic {
		t.Fatalf("partial update wrong: name=%q is_public=%v", got.Name, got.IsPublic)
	}

	resp4, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+id, "Bearer admintok", `{}`))
	if resp4.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for empty update, got %d", resp4.StatusCode)
	}

	resp5, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/bogus", "Bearer admintok", `{"is_public":true}`))
	if resp5.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 for unknown app, got %d", resp5.StatusCode)
	}
}

func TestAdminCreatePublicAppAndSelfRegister(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	resp, err := c.Do(jsonReq(t, "POST", srv.URL+"/apps", "Bearer admintok", `{"name":"openapp","is_public":true}`))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("create public app: err=%v status=%d", err, resp.StatusCode)
	}
	var a struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Token string `json:"token"`
	}
	dataOf(t, resp, &a)
	if a.ID == "" || a.Name != "openapp" || !strings.HasPrefix(a.Token, "pat_") {
		t.Fatalf("bad app resp: %+v", a)
	}
}

func TestAdminRotateInstanceToken(t *testing.T) {
	srv, _, _, instances := newAdminServer(t)
	c := srv.Client()

	id, _, _ := createAppAdmin(t, srv, `{"name":"rotinst"}`)

	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+id+"/instances", "Bearer admintok", `{"label":"svc"}`))
	var k struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	dataOf(t, resp, &k)
	oldToken := k.Token

	resp2, err := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+id+"/instances/"+k.ID+"/rotate-token", "Bearer admintok", ""))
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("rotate: err=%v status=%d", err, resp2.StatusCode)
	}
	var out struct {
		Token string `json:"token"`
	}
	dataOf(t, resp2, &out)
	if !strings.HasPrefix(out.Token, "pit_") || out.Token == oldToken {
		t.Fatalf("bad new token: %q (old %q)", out.Token, oldToken)
	}

	if _, err := instances.Authenticate(t.Context(), oldToken); err == nil {
		t.Fatal("old token should no longer authenticate")
	}
	rec, err := instances.Authenticate(t.Context(), out.Token)
	if err != nil || rec.ID != k.ID {
		t.Fatalf("new token should authenticate to same instance: rec=%+v err=%v", rec, err)
	}

	resp3, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+id+"/instances/bogus/rotate-token", "Bearer admintok", ""))
	if resp3.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 for unknown instance, got %d", resp3.StatusCode)
	}
}

func TestAppSelfManagement(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	id, _, appToken := createAppAdmin(t, srv, `{"name":"selfmgmt"}`)

	t.Run("credentials with app token", func(t *testing.T) {
		body := `{"key_p8":"PEM","key_id":"K1","team_id":"T1","topic":"com.x","production":true}`
		resp, err := c.Do(jsonReq(t, "PUT", srv.URL+"/apps/"+id+"/creds/apns", "Bearer "+appToken, body))
		if err != nil || resp.StatusCode != http.StatusNoContent {
			t.Fatalf("set creds with app token: err=%v status=%d", err, resp.StatusCode)
		}
	})

	t.Run("issue instance with app token", func(t *testing.T) {
		resp, err := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+id+"/instances", "Bearer "+appToken, `{"label":"self"}`))
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("issue instance with app token: err=%v status=%d", err, resp.StatusCode)
		}
		var k struct {
			ID    string `json:"id"`
			Token string `json:"token"`
		}
		dataOf(t, resp, &k)
		if !strings.HasPrefix(k.Token, "pit_") {
			t.Fatalf("bad token: %+v", k)
		}
	})
}

func TestAppSelfManagement_NoToken(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	id, _, _ := createAppAdmin(t, srv, `{"name":"notoken"}`)

	resp, err := c.Do(jsonReq(t, "PUT", srv.URL+"/apps/"+id+"/creds/apns", "", `{}`))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 with no token, got %d", resp.StatusCode)
	}
}

func TestAppSelfManagement_InvalidToken(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	id, _, _ := createAppAdmin(t, srv, `{"name":"badtoken"}`)

	resp, err := c.Do(jsonReq(t, "PUT", srv.URL+"/apps/"+id+"/creds/apns", "Bearer pat_totallywrong", `{}`))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 with invalid token, got %d", resp.StatusCode)
	}
}

func TestAppSelfManagement_CrossAppDenied(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	idA, _, _ := createAppAdmin(t, srv, `{"name":"appA"}`)
	idB, _, appTokenB := createAppAdmin(t, srv, `{"name":"appB"}`)
	_ = idA

	resp, err := c.Do(jsonReq(t, "PUT", srv.URL+"/apps/"+idA+"/creds/apns", "Bearer "+appTokenB, `{}`))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 cross-app denied, got %d (appB token on appA path)", resp.StatusCode)
	}
	_ = idB
}

func TestAppSelfManagement_InstanceOwnership(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	idA, _, appTokenA := createAppAdmin(t, srv, `{"name":"ownerA"}`)
	idB, _, appTokenB := createAppAdmin(t, srv, `{"name":"ownerB"}`)

	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+idA+"/instances", "Bearer admintok", `{"label":"instA"}`))
	var k struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &k)

	resp2, err := c.Do(jsonReq(t, "DELETE", srv.URL+"/apps/"+idB+"/instances/"+k.ID, "Bearer "+appTokenB, ""))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403 for cross-app instance delete, got %d", resp2.StatusCode)
	}

	resp3, err := c.Do(jsonReq(t, "DELETE", srv.URL+"/apps/"+idA+"/instances/"+k.ID, "Bearer "+appTokenA, ""))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp3.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204 for own instance delete, got %d", resp3.StatusCode)
	}
}

func TestAppSelfManagement_AdminAccessAll(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	id, _, _ := createAppAdmin(t, srv, `{"name":"anyapp"}`)

	resp, err := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+id+"/instances", "Bearer admintok", `{"label":"op"}`))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("admin access to app route: err=%v status=%d", err, resp.StatusCode)
	}
}

func TestAppSelfManagement_SetPublicViaAppToken(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	id, _, appToken := createAppAdmin(t, srv, `{"name":"selfpub"}`)

	resp, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+id, "Bearer "+appToken, `{"is_public":true}`))
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("set public via app token: err=%v status=%d", err, resp.StatusCode)
	}
}

func TestAppSelfManagement_RotateToken(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	id, _, oldToken := createAppAdmin(t, srv, `{"name":"rotator"}`)

	resp, err := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+id+"/rotate-token", "Bearer admintok", ""))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("rotate: err=%v status=%d", err, resp.StatusCode)
	}
	var result struct {
		Token string `json:"token"`
	}
	dataOf(t, resp, &result)
	newToken := result.Token
	if !strings.HasPrefix(newToken, "pat_") || newToken == oldToken {
		t.Fatalf("bad new token: %q", newToken)
	}

	resp2, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+id, "Bearer "+oldToken, `{"is_public":false}`))
	if err != nil {
		t.Fatalf("request with old token: %v", err)
	}
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 with old token after rotation, got %d", resp2.StatusCode)
	}

	resp3, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+id, "Bearer "+newToken, `{"is_public":false}`))
	if err != nil || resp3.StatusCode != http.StatusNoContent {
		t.Fatalf("new token should work: err=%v status=%d", err, resp3.StatusCode)
	}
}

func TestAdminSetTurnstile(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()
	id, _, _ := createAppAdmin(t, srv, `{"name":"ts"}`)

	do := func(method, path, body string) int {
		resp, err := c.Do(jsonReq(t, method, srv.URL+path, "Bearer admintok", body))
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		return resp.StatusCode
	}

	if s := do("PUT", "/apps/"+id+"/turnstile", `{"site_key":"s","secret_key":"k"}`); s != http.StatusNoContent {
		t.Fatalf("set: %d", s)
	}
	if s := do("PUT", "/apps/"+id+"/turnstile", `{"site_key":"s"}`); s != http.StatusBadRequest {
		t.Fatalf("missing field: want 400 got %d", s)
	}
	if s := do("DELETE", "/apps/"+id+"/turnstile", ""); s != http.StatusNoContent {
		t.Fatalf("delete: %d", s)
	}
	if s := do("PUT", "/apps/bogus/turnstile", `{"site_key":"s","secret_key":"k"}`); s != http.StatusNotFound {
		t.Fatalf("unknown app: want 404 got %d", s)
	}
}

func TestAdminRotateEndpointKey(t *testing.T) {
	srv, d, _, _ := newAdminServer(t)
	c := srv.Client()
	id, _, _ := createAppAdmin(t, srv, `{"name":"rk"}`)

	// graceful: key_version bumps, min stays
	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+id+"/rotate-endpoint-key", "Bearer admintok", ""))
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("graceful rotate: %d", resp.StatusCode)
	}
	a, _ := d.Queries.GetApp(t.Context(), id)
	if a.KeyVersion != 2 || a.MinKeyVersion != 1 {
		t.Fatalf("graceful: kv=%d min=%d", a.KeyVersion, a.MinKeyVersion)
	}

	// immediate: revoke_old bumps both
	resp2, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+id+"/rotate-endpoint-key", "Bearer admintok", `{"revoke_old":true}`))
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("immediate rotate: %d", resp2.StatusCode)
	}
	a, _ = d.Queries.GetApp(t.Context(), id)
	if a.KeyVersion != 3 || a.MinKeyVersion != 3 {
		t.Fatalf("immediate: kv=%d min=%d", a.KeyVersion, a.MinKeyVersion)
	}

	resp3, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/bogus/rotate-endpoint-key", "Bearer admintok", ""))
	if resp3.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown: %d", resp3.StatusCode)
	}
}
