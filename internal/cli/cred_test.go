package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCredsSetApns(t *testing.T) {
	srv, rec := testServer(t, http.StatusNoContent, ``)
	_, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1",
		"creds", "set", "apns", "--key-p8", "PEMDATA", "--key-id", "K1", "--team-id", "T1", "--topic", "com.x")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "PUT" || rec.Path != "/apps/a1/creds/apns" {
		t.Fatalf("req: %+v", rec)
	}
	if rec.Body != `{"key_id":"K1","key_p8":"PEMDATA","production":false,"team_id":"T1","topic":"com.x"}` {
		t.Fatalf("body: %s", rec.Body)
	}
}

func TestCredsSetApnsProduction(t *testing.T) {
	srv, rec := testServer(t, http.StatusNoContent, ``)
	_, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1",
		"creds", "set", "apns", "--key-p8", "PEMDATA", "--key-id", "K1", "--team-id", "T1", "--topic", "com.x", "--production")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Body != `{"key_id":"K1","key_p8":"PEMDATA","production":true,"team_id":"T1","topic":"com.x"}` {
		t.Fatalf("body: %s", rec.Body)
	}
}

func TestCredsSetApnsAtFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "key.p8")
	if err := os.WriteFile(p, []byte("FILEPEM"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv, rec := testServer(t, http.StatusNoContent, ``)
	_, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1",
		"creds", "set", "apns", "--key-p8", "@"+p, "--key-id", "K1", "--team-id", "T1", "--topic", "com.x")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if want := `"key_p8":"FILEPEM"`; !strings.Contains(rec.Body, want) {
		t.Fatalf("want %s in body, got %s", want, rec.Body)
	}
}

func TestCredsSetWebPushAtFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "vapid.key")
	if err := os.WriteFile(p, []byte("FILEPRIV"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv, rec := testServer(t, http.StatusNoContent, ``)
	_, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1",
		"creds", "set", "webpush", "--vapid-private", "@"+p, "--subject", "mailto:x@y.z")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if want := `"vapid_private_key":"FILEPRIV"`; !strings.Contains(rec.Body, want) {
		t.Fatalf("want %s in body, got %s", want, rec.Body)
	}
}

func TestCredsRm(t *testing.T) {
	srv, rec := testServer(t, http.StatusNoContent, ``)
	if _, err := execute(t, "--base-url", srv.URL, "--token", "t", "--app", "a1", "creds", "rm", "fcm"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.Method != "DELETE" || rec.Path != "/apps/a1/creds/fcm" {
		t.Fatalf("req: %+v", rec)
	}
}
