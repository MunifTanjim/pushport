package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAtAppPlaceholder(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	appID, _, pat := createAppAdmin(t, srv, `{"name":"atapp"}`)

	resp, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/@app", "Bearer "+pat, ""))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("get @app via pat: err=%v status=%d", err, resp.StatusCode)
	}
	var got struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &got)
	if got.ID != appID {
		t.Errorf("@app should resolve to %q, got %q", appID, got.ID)
	}

	// admin token is not bound to one app, so @app → 401.
	resp2, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/@app", "Bearer admintok", ""))
	if err != nil || resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("get @app via admin: err=%v status=%d", err, resp2.StatusCode)
	}
}

func TestGetApp(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	appID, appName, pat := createAppAdmin(t, srv, `{"name":"getapp"}`)

	resp, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID, "Bearer admintok", ""))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("get app: err=%v status=%d", err, resp.StatusCode)
	}
	var got struct {
		ID               string  `json:"id"`
		Name             string  `json:"name"`
		IsPublic         bool    `json:"is_public"`
		KeyVersion       int64   `json:"key_version"`
		MinKeyVersion    int64   `json:"min_key_version"`
		UsagePlanID      *string `json:"usage_plan_id"`
		TurnstileSiteKey *string `json:"turnstile_site_key"`
		CreatedAt        int64   `json:"created_at"`
	}
	dataOf(t, resp, &got)

	if got.ID != appID {
		t.Errorf("want id=%q, got %q", appID, got.ID)
	}
	if got.Name != appName {
		t.Errorf("want name=%q, got %q", appName, got.Name)
	}
	if got.IsPublic {
		t.Error("want is_public=false for new app")
	}
	if got.KeyVersion == 0 {
		t.Error("key_version should be non-zero")
	}
	if got.CreatedAt == 0 {
		t.Error("created_at should be non-zero")
	}

	raw, _ := json.Marshal(got)
	rawStr := string(raw)
	for _, forbidden := range []string{"token", "enc_token_hash", "enc_turnstile_secret"} {
		if strings.Contains(rawStr, forbidden) {
			t.Errorf("response contains forbidden field %q", forbidden)
		}
	}

	resp2, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID, "Bearer "+pat, ""))
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("get app via pat: err=%v status=%d", err, resp2.StatusCode)
	}

	resp3, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/doesnotexist", "Bearer admintok", ""))
	if err != nil || resp3.StatusCode != http.StatusNotFound {
		t.Fatalf("get unknown app: err=%v status=%d", err, resp3.StatusCode)
	}
}

func TestListApps(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	_, _, pat := createAppAdmin(t, srv, `{"name":"listapp1"}`)
	createAppAdmin(t, srv, `{"name":"listapp2"}`)

	resp, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps", "Bearer admintok", ""))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("list apps: err=%v status=%d", err, resp.StatusCode)
	}
	var apps []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	dataOf(t, resp, &apps)
	if len(apps) < 2 {
		t.Fatalf("want at least 2 apps, got %d", len(apps))
	}

	resp2, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps", "Bearer "+pat, ""))
	if err != nil {
		t.Fatalf("list apps with pat: err=%v", err)
	}
	if resp2.StatusCode != http.StatusUnauthorized && resp2.StatusCode != http.StatusForbidden {
		t.Errorf("list apps with pat: want 401/403, got %d", resp2.StatusCode)
	}
}

func TestListAndGetInstances(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	appAID, _, patA := createAppAdmin(t, srv, `{"name":"instapp-a"}`)
	appBID, _, patB := createAppAdmin(t, srv, `{"name":"instapp-b"}`)

	var inst1, inst2 struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	r1, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appAID+"/instances", "Bearer admintok", `{"label":"svc1"}`))
	if r1.StatusCode != http.StatusCreated {
		t.Fatalf("create inst1: status=%d", r1.StatusCode)
	}
	dataOf(t, r1, &inst1)

	r2, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appAID+"/instances", "Bearer admintok", `{"label":"svc2"}`))
	if r2.StatusCode != http.StatusCreated {
		t.Fatalf("create inst2: status=%d", r2.StatusCode)
	}
	dataOf(t, r2, &inst2)

	resp, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appAID+"/instances", "Bearer admintok", ""))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("list instances: err=%v status=%d", err, resp.StatusCode)
	}
	var instances []struct {
		ID          string  `json:"id"`
		AppID       string  `json:"app_id"`
		Label       string  `json:"label"`
		UsagePlanID *string `json:"usage_plan_id"`
		CreatedAt   int64   `json:"created_at"`
	}
	dataOf(t, resp, &instances)
	findInst := func(id string) bool {
		for _, i := range instances {
			if i.ID == id {
				return true
			}
		}
		return false
	}
	if !findInst(inst1.ID) || !findInst(inst2.ID) {
		t.Fatalf("instances not in list: %+v", instances)
	}
	for _, i := range instances {
		raw, _ := json.Marshal(i)
		if strings.Contains(string(raw), "enc_token_hash") {
			t.Errorf("instance list: enc_token_hash leaked in response")
		}
	}

	resp2, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appAID+"/instances", "Bearer "+patA, ""))
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("list instances via pat: err=%v status=%d", err, resp2.StatusCode)
	}

	resp3, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appAID+"/instances/"+inst1.ID, "Bearer admintok", ""))
	if err != nil || resp3.StatusCode != http.StatusOK {
		t.Fatalf("get instance: err=%v status=%d", err, resp3.StatusCode)
	}
	var gotInst struct {
		ID    string `json:"id"`
		AppID string `json:"app_id"`
		Label string `json:"label"`
	}
	dataOf(t, resp3, &gotInst)
	if gotInst.ID != inst1.ID || gotInst.AppID != appAID || gotInst.Label != "svc1" {
		t.Errorf("get instance: got %+v", gotInst)
	}

	resp4, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appAID+"/instances/"+inst1.ID, "Bearer "+patA, ""))
	if err != nil || resp4.StatusCode != http.StatusOK {
		t.Fatalf("get instance via pat: err=%v status=%d", err, resp4.StatusCode)
	}

	resp5, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appAID+"/instances/"+inst1.ID, "Bearer "+patB, ""))
	if err != nil {
		t.Fatalf("cross-app get: err=%v", err)
	}
	if resp5.StatusCode != http.StatusUnauthorized && resp5.StatusCode != http.StatusForbidden {
		t.Errorf("cross-app get: want 401/403, got %d", resp5.StatusCode)
	}

	resp6, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appBID+"/instances/"+inst1.ID, "Bearer admintok", ""))
	if err != nil {
		t.Fatalf("wrong-app-path get: err=%v", err)
	}
	if resp6.StatusCode != http.StatusForbidden {
		t.Errorf("wrong app_id path: want 403, got %d", resp6.StatusCode)
	}

	resp7, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appAID+"/instances/nosuchinstance", "Bearer admintok", ""))
	if err != nil || resp7.StatusCode != http.StatusNotFound {
		t.Fatalf("get unknown instance: err=%v status=%d", err, resp7.StatusCode)
	}
}

func TestListCreds(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	appID, _, pat := createAppAdmin(t, srv, `{"name":"credsapp"}`)

	resp, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/creds", "Bearer admintok", ""))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("list creds empty: err=%v status=%d", err, resp.StatusCode)
	}
	var got struct {
		Transports []string `json:"transports"`
	}
	dataOf(t, resp, &got)
	if len(got.Transports) != 0 {
		t.Fatalf("want empty transports, got %v", got.Transports)
	}

	apns := `{"key_p8":"PEM","key_id":"K1","team_id":"T1","topic":"com.x","production":true}`
	r2, _ := c.Do(jsonReq(t, "PUT", srv.URL+"/apps/"+appID+"/creds/apns", "Bearer admintok", apns))
	if r2.StatusCode != http.StatusNoContent {
		t.Fatalf("set apns: status=%d", r2.StatusCode)
	}

	resp3, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/creds", "Bearer admintok", ""))
	if err != nil || resp3.StatusCode != http.StatusOK {
		t.Fatalf("list creds after set: err=%v status=%d", err, resp3.StatusCode)
	}
	var got3 struct {
		Transports []string `json:"transports"`
	}
	dataOf(t, resp3, &got3)
	if len(got3.Transports) != 1 || got3.Transports[0] != "apns" {
		t.Fatalf("want [apns], got %v", got3.Transports)
	}
	raw3, _ := json.Marshal(got3)
	if strings.Contains(string(raw3), "enc_blob") {
		t.Error("listCreds: enc_blob leaked in response")
	}

	resp4, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/creds", "Bearer "+pat, ""))
	if err != nil || resp4.StatusCode != http.StatusOK {
		t.Fatalf("list creds via pat: err=%v status=%d", err, resp4.StatusCode)
	}

	resp5, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/nope/creds", "Bearer admintok", ""))
	if err != nil || resp5.StatusCode != http.StatusNotFound {
		t.Fatalf("list creds unknown app: err=%v status=%d", err, resp5.StatusCode)
	}
}

func TestGetTurnstile(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	appID, _, pat := createAppAdmin(t, srv, `{"name":"tsapp"}`)

	resp, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/turnstile", "Bearer admintok", ""))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("get turnstile unconfigured: err=%v status=%d", err, resp.StatusCode)
	}
	var got struct {
		Configured bool    `json:"configured"`
		SiteKey    *string `json:"site_key"`
	}
	dataOf(t, resp, &got)
	if got.Configured {
		t.Error("want configured=false initially")
	}
	if got.SiteKey != nil {
		t.Errorf("want site_key=nil, got %v", got.SiteKey)
	}

	r2, _ := c.Do(jsonReq(t, "PUT", srv.URL+"/apps/"+appID+"/turnstile", "Bearer admintok",
		`{"site_key":"my-site-key","secret_key":"my-secret"}`))
	if r2.StatusCode != http.StatusNoContent {
		t.Fatalf("set turnstile: status=%d", r2.StatusCode)
	}

	resp3, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/turnstile", "Bearer admintok", ""))
	if err != nil || resp3.StatusCode != http.StatusOK {
		t.Fatalf("get turnstile configured: err=%v status=%d", err, resp3.StatusCode)
	}
	var got3 struct {
		Configured bool    `json:"configured"`
		SiteKey    *string `json:"site_key"`
	}
	dataOf(t, resp3, &got3)
	if !got3.Configured {
		t.Error("want configured=true after set")
	}
	if got3.SiteKey == nil || *got3.SiteKey != "my-site-key" {
		t.Errorf("want site_key=my-site-key, got %v", got3.SiteKey)
	}

	rawBytes, _ := json.Marshal(got3)
	rawStr := string(rawBytes)
	if strings.Contains(rawStr, "my-secret") || strings.Contains(rawStr, "enc_turnstile_secret") || strings.Contains(rawStr, "secret_key") {
		t.Errorf("turnstile response leaks secret: %s", rawStr)
	}

	resp4, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/turnstile", "Bearer "+pat, ""))
	if err != nil || resp4.StatusCode != http.StatusOK {
		t.Fatalf("get turnstile via pat: err=%v status=%d", err, resp4.StatusCode)
	}

	resp5, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/doesnotexist/turnstile", "Bearer admintok", ""))
	if err != nil || resp5.StatusCode != http.StatusNotFound {
		t.Fatalf("get turnstile unknown app: err=%v status=%d", err, resp5.StatusCode)
	}
}

func TestGetAppUsagePlan(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	r1, _ := c.Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok",
		`{"name":"getplan","push_per_min":50,"push_daily_quota":0}`))
	if r1.StatusCode != http.StatusCreated {
		t.Fatalf("create plan: status=%d", r1.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	dataOf(t, r1, &created)

	resp, err := c.Do(jsonReq(t, "GET", srv.URL+"/usage-plans/"+created.ID, "Bearer admintok", ""))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("get app plan: err=%v status=%d", err, resp.StatusCode)
	}
	var got struct {
		ID            string  `json:"id"`
		Name          *string `json:"name"`
		AppPushPerMin *int    `json:"push_per_min"`
	}
	dataOf(t, resp, &got)
	if got.ID != created.ID {
		t.Errorf("want id=%q, got %q", created.ID, got.ID)
	}
	if got.Name == nil || *got.Name != "getplan" {
		t.Errorf("want name=getplan, got %v", got.Name)
	}
	if got.AppPushPerMin == nil || *got.AppPushPerMin != 50 {
		t.Errorf("want push_per_min=50, got %v", got.AppPushPerMin)
	}

	_, _, pat := createAppAdmin(t, srv, `{"name":"patapp"}`)
	resp2, err := c.Do(jsonReq(t, "GET", srv.URL+"/usage-plans/"+created.ID, "Bearer "+pat, ""))
	if err != nil {
		t.Fatalf("get plan with pat: err=%v", err)
	}
	if resp2.StatusCode != http.StatusUnauthorized && resp2.StatusCode != http.StatusForbidden {
		t.Errorf("get app plan with pat: want 401/403, got %d", resp2.StatusCode)
	}

	resp3, err := c.Do(jsonReq(t, "GET", srv.URL+"/usage-plans/doesnotexist", "Bearer admintok", ""))
	if err != nil || resp3.StatusCode != http.StatusNotFound {
		t.Fatalf("get unknown plan: err=%v status=%d", err, resp3.StatusCode)
	}
}

func TestGetInstanceUsagePlan(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	appAID, _, patA := createAppAdmin(t, srv, `{"name":"iplanA"}`)
	appBID, _, _ := createAppAdmin(t, srv, `{"name":"iplanB"}`)

	r1, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appAID+"/usage-plans", "Bearer admintok",
		`{"name":"myplan","push_per_min":20,"push_burst":4,"push_daily_quota":0}`))
	if r1.StatusCode != http.StatusCreated {
		t.Fatalf("create instance plan: status=%d", r1.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	dataOf(t, r1, &created)

	resp, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appAID+"/usage-plans/"+created.ID, "Bearer admintok", ""))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("get instance plan: err=%v status=%d", err, resp.StatusCode)
	}
	var got struct {
		ID                 string  `json:"id"`
		Name               *string `json:"name"`
		InstancePushPerMin *int    `json:"push_per_min"`
		InstancePushBurst  *int    `json:"push_burst"`
	}
	dataOf(t, resp, &got)
	if got.ID != created.ID {
		t.Errorf("want id=%q, got %q", created.ID, got.ID)
	}
	if got.Name == nil || *got.Name != "myplan" {
		t.Errorf("want name=myplan, got %v", got.Name)
	}

	resp2, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appAID+"/usage-plans/"+created.ID, "Bearer "+patA, ""))
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("get instance plan via pat: err=%v status=%d", err, resp2.StatusCode)
	}

	// cross-app fetch (appA's plan via appB's path) → 404, don't leak existence.
	resp3, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appBID+"/usage-plans/"+created.ID, "Bearer admintok", ""))
	if err != nil || resp3.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-app instance plan: err=%v status=%d", err, resp3.StatusCode)
	}

	resp4, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appAID+"/usage-plans/doesnotexist", "Bearer admintok", ""))
	if err != nil || resp4.StatusCode != http.StatusNotFound {
		t.Fatalf("get unknown instance plan: err=%v status=%d", err, resp4.StatusCode)
	}
}
