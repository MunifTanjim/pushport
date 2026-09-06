package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestOutputValidationRejectsInvalidFormat(t *testing.T) {
	srv, _ := testServer(t, http.StatusOK, `{"request_id":"r","data":{"id":"a1","name":"test","is_public":false,"created_at":1}}`)
	_, err := execute(t, "--base-url", srv.URL, "--token", "t", "-o", "bogus", "app", "get")
	if err == nil {
		t.Fatal("want error for invalid output format, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --output") || !strings.Contains(err.Error(), "table or json") {
		t.Fatalf("want error mentioning 'invalid --output' and 'table or json', got: %v", err)
	}
}

func TestOutputValidationRejectsYaml(t *testing.T) {
	srv, _ := testServer(t, http.StatusOK, `{"request_id":"r","data":{"id":"a1","name":"test","is_public":false,"created_at":1}}`)
	_, err := execute(t, "--base-url", srv.URL, "--token", "t", "--output", "yaml", "app", "get")
	if err == nil {
		t.Fatal("want error for yaml output format, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --output") {
		t.Fatalf("want 'invalid --output' in error, got: %v", err)
	}
}

func TestOutputValidationAcceptsTable(t *testing.T) {
	srv, _ := testServer(t, http.StatusOK, `{"request_id":"r","data":{"id":"a1","name":"test","is_public":false,"created_at":1,"key_version":1,"min_key_version":1}}`)
	out, err := execute(t, "--base-url", srv.URL, "--token", "pat_t", "-o", "table", "app", "get")
	if err != nil {
		t.Fatalf("execute with -o table: %v", err)
	}
	if !strings.Contains(out, "a1") || !strings.Contains(out, "test") {
		t.Fatalf("want table output with id and name, got: %s", out)
	}
}

func TestOutputValidationAcceptsJson(t *testing.T) {
	srv, _ := testServer(t, http.StatusOK, `{"request_id":"r","data":{"id":"a1","name":"test","is_public":false,"created_at":1,"key_version":1,"min_key_version":1}}`)
	out, err := execute(t, "--base-url", srv.URL, "--token", "pat_t", "-o", "json", "app", "get")
	if err != nil {
		t.Fatalf("execute with -o json: %v", err)
	}
	if !strings.Contains(out, "\"id\"") || !strings.Contains(out, "a1") {
		t.Fatalf("want json output, got: %s", out)
	}
}

func TestOutputValidationDefaultsToTable(t *testing.T) {
	srv, _ := testServer(t, http.StatusOK, `{"request_id":"r","data":{"id":"a1","name":"test","is_public":false,"created_at":1,"key_version":1,"min_key_version":1}}`)
	out, err := execute(t, "--base-url", srv.URL, "--token", "pat_t", "app", "get")
	if err != nil {
		t.Fatalf("execute with default output: %v", err)
	}
	if !strings.Contains(out, "a1") || !strings.Contains(out, "test") {
		t.Fatalf("want table output by default, got: %s", out)
	}
}
