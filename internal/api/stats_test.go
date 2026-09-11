package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/ratelimit"
)

// GET /stats is admin-only: it reports today's durable push counters plus
// entity totals for the dashboard.

func TestStatsRequiresAdmin(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	resp, err := c.Do(jsonReq(t, "GET", srv.URL+"/stats", "", ""))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d", resp.StatusCode)
	}
	resp2, err := c.Do(jsonReq(t, "GET", srv.URL+"/stats", "Bearer wrongtok", ""))
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad token: want 401, got %d", resp2.StatusCode)
	}
}

func TestStats(t *testing.T) {
	srv, d, _, _ := newAdminServer(t)
	c := srv.Client()

	app1, _, _ := createAppAdmin(t, srv, `{"name":"alpha"}`)
	app2, _, _ := createAppAdmin(t, srv, `{"name":"beta"}`)

	resp, err := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+app1+"/instances", "Bearer admintok", `{"label":"i1"}`))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("create instance: err=%v status=%d", err, resp.StatusCode)
	}
	var inst struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &inst)

	// Seed today's durable counters directly (the harness does not run the push
	// path). One row uses an unknown scope format and must not break the split.
	day := db.NewDate(time.Now())
	seed := []db.UpsertUsageCounterParams{
		{Scope: ratelimit.ScopeApp + app1, Date: day, Count: 100},
		{Scope: ratelimit.ScopeApp + app2, Date: day, Count: 23},
		{Scope: ratelimit.ScopeInstance + inst.ID, Date: day, Count: 50},
		{Scope: "mystery:xyz", Date: day, Count: 7},
	}
	for _, p := range seed {
		if err := d.Queries.UpsertUsageCounter(t.Context(), p); err != nil {
			t.Fatalf("seed counter: %v", err)
		}
	}

	resp2, err := c.Do(jsonReq(t, "GET", srv.URL+"/stats", "Bearer admintok", ""))
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("get stats: err=%v status=%d", err, resp2.StatusCode)
	}
	var s struct {
		Date             string           `json:"date"`
		TotalApps        int64            `json:"total_apps"`
		TotalInstances   int64            `json:"total_instances"`
		TotalPushes      int64            `json:"total_pushes"`
		PushesByApp      map[string]int64 `json:"pushes_by_app"`
		PushesByInstance map[string]int64 `json:"pushes_by_instance"`
	}
	dataOf(t, resp2, &s)

	if s.Date != day.String() {
		t.Fatalf("want date=%s, got %s", day.String(), s.Date)
	}
	if s.TotalApps != 2 || s.TotalInstances != 1 {
		t.Fatalf("want totals apps=2 instances=1, got %d/%d", s.TotalApps, s.TotalInstances)
	}
	// Total is the sum of app-scope counters only (100+23); instance-scope
	// counters mirror the same pushes and the unknown scope is ignored.
	if s.TotalPushes != 123 {
		t.Fatalf("want total_pushes=123, got %d", s.TotalPushes)
	}
	if s.PushesByApp[app1] != 100 || s.PushesByApp[app2] != 23 || len(s.PushesByApp) != 2 {
		t.Fatalf("bad pushes_by_app: %v", s.PushesByApp)
	}
	if s.PushesByInstance[inst.ID] != 50 || len(s.PushesByInstance) != 1 {
		t.Fatalf("bad pushes_by_instance: %v", s.PushesByInstance)
	}
}

// GET /apps/{app_id}/stats is app-scoped: an app maintainer sees only its own
// app's counters (via @app), the admin can read any app by id, and an app
// token cannot read a different app.
func TestAppStats(t *testing.T) {
	srv, d, _, _ := newAdminServer(t)
	c := srv.Client()

	app1, _, tok1 := createAppAdmin(t, srv, `{"name":"alpha"}`)
	app2, _, _ := createAppAdmin(t, srv, `{"name":"beta"}`)

	resp, err := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+app1+"/instances", "Bearer admintok", `{"label":"i1"}`))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("create instance: err=%v status=%d", err, resp.StatusCode)
	}
	var inst struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &inst)

	// Seed today's durable counters (the harness does not run the push path). The
	// unknown-scope row must not leak into the app-scoped split.
	day := db.NewDate(time.Now())
	seed := []db.UpsertUsageCounterParams{
		{Scope: ratelimit.ScopeApp + app1, Date: day, Count: 100},
		{Scope: ratelimit.ScopeApp + app2, Date: day, Count: 23},
		{Scope: ratelimit.ScopeInstance + inst.ID, Date: day, Count: 50},
		{Scope: "mystery:xyz", Date: day, Count: 7},
	}
	for _, p := range seed {
		if err := d.Queries.UpsertUsageCounter(t.Context(), p); err != nil {
			t.Fatalf("seed counter: %v", err)
		}
	}

	type appStats struct {
		Date             string           `json:"date"`
		TotalInstances   int64            `json:"total_instances"`
		TotalPushes      int64            `json:"total_pushes"`
		PushesByInstance map[string]int64 `json:"pushes_by_instance"`
	}

	resp2, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/@app/stats", "Bearer "+tok1, ""))
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("app stats: err=%v status=%d", err, resp2.StatusCode)
	}
	var s appStats
	dataOf(t, resp2, &s)
	if s.Date != day.String() {
		t.Fatalf("want date=%s, got %s", day.String(), s.Date)
	}
	if s.TotalInstances != 1 {
		t.Fatalf("want total_instances=1, got %d", s.TotalInstances)
	}
	if s.TotalPushes != 100 {
		t.Fatalf("want total_pushes=100 (app1's own counter, app2 excluded), got %d", s.TotalPushes)
	}
	if s.PushesByInstance[inst.ID] != 50 || len(s.PushesByInstance) != 1 {
		t.Fatalf("bad pushes_by_instance: %v", s.PushesByInstance)
	}

	resp3, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+app2+"/stats", "Bearer admintok", ""))
	if err != nil || resp3.StatusCode != http.StatusOK {
		t.Fatalf("admin app stats: err=%v status=%d", err, resp3.StatusCode)
	}
	var s2 appStats
	dataOf(t, resp3, &s2)
	if s2.TotalPushes != 23 || s2.TotalInstances != 0 || len(s2.PushesByInstance) != 0 {
		t.Fatalf("bad app2 stats: %+v", s2)
	}

	resp4, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+app2+"/stats", "Bearer "+tok1, ""))
	if err != nil {
		t.Fatal(err)
	}
	if resp4.StatusCode == http.StatusOK {
		t.Fatalf("cross-app read should be rejected, got 200")
	}
}
