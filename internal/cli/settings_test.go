package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestSettingsSetNoFlagsErrors(t *testing.T) {
	srv, rec := testServer(t, http.StatusNoContent, ``)
	_, err := execute(t, "--base-url", srv.URL, "--token", "admintok", "settings", "set")
	if err == nil || !strings.Contains(err.Error(), "nothing to update") {
		t.Fatalf("want 'nothing to update' error, got %v", err)
	}
	if rec.Method != "" {
		t.Fatalf("expected no request to be sent, got %s %s", rec.Method, rec.Path)
	}
}
