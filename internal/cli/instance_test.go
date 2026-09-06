package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestInstanceCreatePrintsToken(t *testing.T) {
	srv, rec := testServer(t, http.StatusCreated, `{"request_id":"r","data":{"id":"i1","token":"pit_secret"}}`)
	out, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1", "instance", "create", "--label", "svc")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "POST" || rec.Path != "/apps/a1/instances" || rec.Body != `{"label":"svc"}` {
		t.Fatalf("req: %+v body=%s", rec, rec.Body)
	}
	if !strings.Contains(out, "pit_secret") {
		t.Fatalf("out: %s", out)
	}
}

func TestInstanceUsagePlanAssign(t *testing.T) {
	srv, rec := testServer(t, http.StatusNoContent, ``)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1",
		"instance", "update", "i1", "--usage-plan-id", "p1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "PATCH" || rec.Path != "/apps/a1/instances/i1" || rec.Body != `{"usage_plan_id":"p1"}` {
		t.Fatalf("req: %+v body=%s", rec, rec.Body)
	}
}

func TestInstanceUpdateLabel(t *testing.T) {
	srv, rec := testServer(t, http.StatusNoContent, ``)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1",
		"instance", "update", "i1", "--label", "prod"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "PATCH" || rec.Path != "/apps/a1/instances/i1" || rec.Body != `{"label":"prod"}` {
		t.Fatalf("req: %+v body=%s", rec, rec.Body)
	}
}

func TestInstanceList(t *testing.T) {
	srv, rec := testServer(t, http.StatusOK, `{"request_id":"r","data":[{"id":"i1","label":"prod","usage_plan_id":"p1"},{"id":"i2","label":"test","usage_plan_id":null}]}`)
	out, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1", "instance", "list")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "GET" || rec.Path != "/apps/a1/instances" {
		t.Fatalf("req: %+v", rec)
	}
	if !strings.Contains(out, "i1") || !strings.Contains(out, "prod") {
		t.Fatalf("out missing instance: %s", out)
	}
}

func TestInstanceGet(t *testing.T) {
	srv, rec := testServer(t, http.StatusOK, `{"request_id":"r","data":{"id":"i1","app_id":"a1","label":"svc"}}`)
	out, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1", "instance", "get", "i1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "GET" || rec.Path != "/apps/a1/instances/i1" {
		t.Fatalf("req: %+v", rec)
	}
	if !strings.Contains(out, "a1") {
		t.Fatalf("out missing app_id: %s", out)
	}
	if !strings.Contains(out, "svc") {
		t.Fatalf("out missing label: %s", out)
	}
}

func TestInstanceDelete(t *testing.T) {
	srv, rec := testServer(t, http.StatusNoContent, ``)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1", "instance", "delete", "i1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "DELETE" || rec.Path != "/apps/a1/instances/i1" {
		t.Fatalf("req: %+v", rec)
	}
}

func TestInstanceRotateToken(t *testing.T) {
	srv, rec := testServer(t, http.StatusOK, `{"request_id":"r","data":{"token":"new_token_123"}}`)
	out, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1", "instance", "rotate-token", "i1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "POST" || rec.Path != "/apps/a1/instances/i1/rotate-token" {
		t.Fatalf("req: %+v", rec)
	}
	if !strings.Contains(out, "new_token_123") {
		t.Fatalf("out missing token: %s", out)
	}
}

func TestInstanceUsagePlanAssignScoped(t *testing.T) {
	srv, rec := testServer(t, http.StatusCreated, `{"request_id":"r","data":{"id":"p1","name":"n","instance_id":"i1"}}`)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1",
		"usage-plan", "instance", "create", "--name", "n", "--instance-id", "i1", "--per-min", "20", "--burst", "1", "--daily-quota", "5"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "POST" || rec.Path != "/apps/a1/usage-plans" ||
		rec.Body != `{"instance_id":"i1","name":"n","push_burst":1,"push_daily_quota":5,"push_per_min":20}` {
		t.Fatalf("req: %+v body=%s", rec, rec.Body)
	}
}
