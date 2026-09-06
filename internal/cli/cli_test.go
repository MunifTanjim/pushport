package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recordedReq struct {
	Method string
	Path   string
	Auth   string
	Body   string
}

func testServer(t *testing.T, status int, respBody string) (*httptest.Server, *recordedReq) {
	t.Helper()
	rec := &recordedReq{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec.Method, rec.Path, rec.Auth, rec.Body = r.Method, r.URL.Path, r.Header.Get("Authorization"), string(b)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	// reset persistent flag vars mutated by cobra between runs
	flagBaseURL, flagToken, flagApp, flagOutput, flagTimeout = "", "", "@app", "table", 0
	root := newRootCmd("test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestRequireConcreteAppGuard(t *testing.T) {
	// Admin token + @app must fail before any request is made.
	_, err := execute(t, "--token", "admintok", "instance", "list")
	if err == nil || !strings.Contains(err.Error(), "concrete --app") {
		t.Fatalf("want concrete-app error for admin token with @app, got %v", err)
	}

	// An app token (pat_) may use @app: the guard passes and the request goes out.
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"request_id":"r1","data":[]}`))
	}))
	t.Cleanup(srv.Close)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "pat_abc", "instance", "list"); err != nil {
		t.Fatalf("pat_ token with @app should pass the guard, got %v", err)
	}
	if path != "/apps/@app/instances" {
		t.Fatalf("want request to /apps/@app/instances, got %s", path)
	}

	if _, err := execute(t, "--base-url", srv.URL, "--token", "admintok", "--app", "a1", "instance", "list"); err != nil {
		t.Fatalf("concrete --app should pass the guard, got %v", err)
	}
}
