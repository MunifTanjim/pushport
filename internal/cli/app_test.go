package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestAppCreate(t *testing.T) {
	srv, rec := testServer(t, http.StatusCreated, `{"request_id":"r","data":{"id":"a1","name":"demo","token":"pat_secret"}}`)
	out, err := execute(t, "--base-url", srv.URL, "--token", "admintok", "app", "create", "--name", "demo", "--public")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "POST" || rec.Path != "/apps" || rec.Auth != "Bearer admintok" {
		t.Fatalf("req: %+v", rec)
	}
	if rec.Body != `{"name":"demo","is_public":true}` {
		t.Fatalf("body: %s", rec.Body)
	}
	if !strings.Contains(out, "pat_secret") || !strings.Contains(out, "a1") {
		t.Fatalf("out: %s", out)
	}
}

func TestAppUpdatePartialOnlyChangedFields(t *testing.T) {
	srv, rec := testServer(t, http.StatusNoContent, ``)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1", "app", "update", "--private"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "PATCH" || rec.Path != "/apps/a1" || rec.Body != `{"is_public":false}` {
		t.Fatalf("req: %+v body=%s", rec, rec.Body)
	}
}

func TestAppGetUsesAtAppDefault(t *testing.T) {
	srv, rec := testServer(t, http.StatusOK, `{"request_id":"r","data":{"id":"a1","name":"demo","is_public":false,"created_at":1}}`)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "pat_x", "app", "get"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Path != "/apps/@app" {
		t.Fatalf("want /apps/@app, got %s", rec.Path)
	}
}

func TestAppUpdateAssignsUsagePlan(t *testing.T) {
	srv, rec := testServer(t, http.StatusNoContent, ``)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "admintok", "--app", "a1",
		"app", "update", "--usage-plan-id", "p1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "PATCH" || rec.Path != "/apps/a1" || rec.Body != `{"usage_plan_id":"p1"}` {
		t.Fatalf("req: %+v body=%s", rec, rec.Body)
	}
}

func TestAppUpdateUnassignsUsagePlan(t *testing.T) {
	srv, rec := testServer(t, http.StatusNoContent, ``)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "admintok", "--app", "a1",
		"app", "update", "--usage-plan-id", ""); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Body != `{"usage_plan_id":""}` {
		t.Fatalf("body: %s", rec.Body)
	}
}
