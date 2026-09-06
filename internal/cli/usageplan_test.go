package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestUsagePlanUpdateNoFlagsErrors(t *testing.T) {
	t.Run("app", func(t *testing.T) {
		srv, rec := testServer(t, http.StatusNoContent, ``)
		_, err := execute(t, "--base-url", srv.URL, "--token", "admintok", "usage-plan", "app", "update", "p1")
		if err == nil || !strings.Contains(err.Error(), "nothing to update") {
			t.Fatalf("want 'nothing to update' error, got %v", err)
		}
		if rec.Method != "" {
			t.Fatalf("expected no request to be sent, got %s %s", rec.Method, rec.Path)
		}
	})
	t.Run("instance", func(t *testing.T) {
		srv, rec := testServer(t, http.StatusNoContent, ``)
		_, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1", "usage-plan", "instance", "update", "p1")
		if err == nil || !strings.Contains(err.Error(), "nothing to update") {
			t.Fatalf("want 'nothing to update' error, got %v", err)
		}
		if rec.Method != "" {
			t.Fatalf("expected no request to be sent, got %s %s", rec.Method, rec.Path)
		}
	})
}

func TestUsagePlanAppCreate(t *testing.T) {
	srv, rec := testServer(t, http.StatusCreated, `{"request_id":"r","data":{"id":"p1","name":"gold","is_default":false}}`)
	_, err := execute(t, "--base-url", srv.URL, "--token", "admintok",
		"usage-plan", "app", "create", "--name", "gold", "--per-min", "100", "--daily-quota", "200")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "POST" || rec.Path != "/usage-plans" || rec.Body != `{"name":"gold","push_daily_quota":200,"push_per_min":100}` {
		t.Fatalf("req: %+v body=%s", rec, rec.Body)
	}
}

func TestUsagePlanAppCreateScoped(t *testing.T) {
	srv, rec := testServer(t, http.StatusCreated, `{"request_id":"r","data":{"id":"p1","name":"bespoke","app_id":"a2"}}`)
	_, err := execute(t, "--base-url", srv.URL, "--token", "admintok",
		"usage-plan", "app", "create", "--name", "bespoke", "--app-id", "a2", "--per-min", "10", "--daily-quota", "5")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "POST" || rec.Path != "/usage-plans" ||
		rec.Body != `{"app_id":"a2","name":"bespoke","push_daily_quota":5,"push_per_min":10}` {
		t.Fatalf("req: %+v body=%s", rec, rec.Body)
	}
}

func TestUsagePlanInstanceListUsesApp(t *testing.T) {
	srv, rec := testServer(t, http.StatusOK, `{"request_id":"r","data":[]}`)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1", "usage-plan", "instance", "list"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Path != "/apps/a1/usage-plans" {
		t.Fatalf("path: %s", rec.Path)
	}
}

func TestUsagePlanInstanceUpdateOnlyChanged(t *testing.T) {
	srv, rec := testServer(t, http.StatusNoContent, ``)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1",
		"usage-plan", "instance", "update", "p1", "--burst", "3"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "PATCH" || rec.Path != "/apps/a1/usage-plans/p1" || rec.Body != `{"push_burst":3}` {
		t.Fatalf("req: %+v body=%s", rec, rec.Body)
	}
}

func TestUsagePlanAppListShowsDefault(t *testing.T) {
	srv, _ := testServer(t, http.StatusOK, `{"request_id":"r","data":[{"id":"p1","name":"default","is_default":true}]}`)
	out, err := execute(t, "--base-url", srv.URL, "--token", "admintok", "usage-plan", "app", "list")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "default") {
		t.Fatalf("out: %s", out)
	}
}

func TestUsagePlanInstanceCreateDailyQuota(t *testing.T) {
	srv, rec := testServer(t, http.StatusCreated, `{"request_id":"r","data":{"id":"p1","name":"n","is_default":false}}`)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1",
		"usage-plan", "instance", "create", "--name", "n", "--per-min", "60", "--burst", "1", "--daily-quota", "500"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "POST" || rec.Path != "/apps/a1/usage-plans" ||
		rec.Body != `{"name":"n","push_burst":1,"push_daily_quota":500,"push_per_min":60}` {
		t.Fatalf("req: %+v body=%s", rec, rec.Body)
	}
}
