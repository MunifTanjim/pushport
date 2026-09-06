package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MunifTanjim/pushport/internal/api"
)

func corsOKHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestWithCORS_AllowedOrigin(t *testing.T) {
	h := api.WithCORS([]string{"https://console.example.com"}, corsOKHandler())

	req := httptest.NewRequest(http.MethodGet, "/apps", nil)
	req.Header.Set("Origin", "https://console.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://console.example.com" {
		t.Fatalf("Allow-Origin = %q, want echoed origin", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary = %q, want Origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Allow-Credentials = %q, want unset", got)
	}
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("non-preflight request should pass through, got %d %q", rec.Code, rec.Body.String())
	}
}

func TestWithCORS_DisallowedOrigin(t *testing.T) {
	h := api.WithCORS([]string{"https://console.example.com"}, corsOKHandler())

	req := httptest.NewRequest(http.MethodGet, "/apps", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Allow-Origin = %q, want unset for disallowed origin", got)
	}
	// Vary: Origin must be set even for a non-allowlisted origin so caches key on
	// Origin and can't serve this headerless response to an allowed origin.
	if vary := rec.Header().Values("Vary"); len(vary) != 1 || vary[0] != "Origin" {
		t.Fatalf("Vary = %v, want exactly [Origin] for disallowed origin", vary)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("request should still pass through, got %d", rec.Code)
	}
}

func TestWithCORS_Preflight(t *testing.T) {
	h := api.WithCORS([]string{"https://console.example.com"}, corsOKHandler())

	req := httptest.NewRequest(http.MethodOptions, "/apps", nil)
	req.Header.Set("Origin", "https://console.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("Allow-Methods should be set on preflight")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatalf("Allow-Headers should be set on preflight")
	}
	if rec.Body.String() != "" {
		t.Fatalf("preflight should not reach next handler, body = %q", rec.Body.String())
	}
}

func TestWithCORS_EmptyAllowlistIsNoop(t *testing.T) {
	h := api.WithCORS(nil, corsOKHandler())

	req := httptest.NewRequest(http.MethodGet, "/apps", nil)
	req.Header.Set("Origin", "https://console.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("empty allowlist should add no CORS headers, got %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("request should pass through, got %d", rec.Code)
	}
}
