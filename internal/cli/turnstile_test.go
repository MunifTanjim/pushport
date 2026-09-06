package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTurnstileSet(t *testing.T) {
	srv, rec := testServer(t, http.StatusNoContent, ``)
	_, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1",
		"turnstile", "set", "--site-key", "SK", "--secret-key", "SEC")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "PUT" || rec.Path != "/apps/a1/turnstile" || rec.Body != `{"secret_key":"SEC","site_key":"SK"}` {
		t.Fatalf("req: %+v body=%s", rec, rec.Body)
	}
}

func TestTurnstileSetAtFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "secret")
	if err := os.WriteFile(p, []byte("FILESECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv, rec := testServer(t, http.StatusNoContent, ``)
	_, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1",
		"turnstile", "set", "--site-key", "SK", "--secret-key", "@"+p)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if want := `"secret_key":"FILESECRET"`; !strings.Contains(rec.Body, want) {
		t.Fatalf("want %s in body, got %s", want, rec.Body)
	}
}

func TestTurnstileRm(t *testing.T) {
	srv, rec := testServer(t, http.StatusNoContent, ``)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1", "turnstile", "rm"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "DELETE" || rec.Path != "/apps/a1/turnstile" {
		t.Fatalf("req: %+v", rec)
	}
}

func TestTurnstileGet(t *testing.T) {
	srv, _ := testServer(t, http.StatusOK, `{"request_id":"r","data":{"configured":true,"site_key":"mykey"}}`)
	out, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1", "turnstile", "get")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "mykey") {
		t.Fatalf("expected site key in output, got: %s", out)
	}
	if !strings.Contains(out, "SITE_KEY") || !strings.Contains(out, "CONFIGURED") {
		t.Fatalf("expected column headers in output, got: %s", out)
	}
}
