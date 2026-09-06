package api

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	t.Run("configured header is read from a public peer", func(t *testing.T) {
		SetClientIPHeader("CF-Connecting-IP")
		t.Cleanup(func() { SetClientIPHeader("") })
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "198.51.100.9:5555"
		req.Header.Set("CF-Connecting-IP", "203.0.113.7")
		if got := clientIP(req); got != "203.0.113.7" {
			t.Fatalf("want 203.0.113.7, got %q", got)
		}
	})

	t.Run("configured header takes precedence over X-Forwarded-For", func(t *testing.T) {
		SetClientIPHeader("CF-Connecting-IP")
		t.Cleanup(func() { SetClientIPHeader("") })
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:5555"
		req.Header.Set("CF-Connecting-IP", "203.0.113.7")
		req.Header.Set("X-Forwarded-For", "198.51.100.50")
		if got := clientIP(req); got != "203.0.113.7" {
			t.Fatalf("want 203.0.113.7, got %q", got)
		}
	})

	t.Run("configured header absent falls back to RemoteAddr (no XFF fallthrough)", func(t *testing.T) {
		SetClientIPHeader("CF-Connecting-IP")
		t.Cleanup(func() { SetClientIPHeader("") })
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:5555"
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		if got := clientIP(req); got != "10.0.0.1" {
			t.Fatalf("want 10.0.0.1, got %q", got)
		}
	})

	t.Run("private peer honors X-Forwarded-For (right-most entry)", func(t *testing.T) {
		SetClientIPHeader("")
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:5555"
		req.Header.Set("X-Forwarded-For", "1.2.3.4, 203.0.113.7") // client-prepended 1.2.3.4 ignored
		if got := clientIP(req); got != "203.0.113.7" {
			t.Fatalf("want 203.0.113.7, got %q", got)
		}
	})

	t.Run("loopback peer honors X-Forwarded-For", func(t *testing.T) {
		SetClientIPHeader("")
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:5555"
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		if got := clientIP(req); got != "203.0.113.7" {
			t.Fatalf("want 203.0.113.7, got %q", got)
		}
	})

	t.Run("public peer ignores X-Forwarded-For", func(t *testing.T) {
		SetClientIPHeader("")
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "198.51.100.9:5555"
		req.Header.Set("X-Forwarded-For", "203.0.113.7") // spoof attempt, must be ignored
		if got := clientIP(req); got != "198.51.100.9" {
			t.Fatalf("want 198.51.100.9, got %q", got)
		}
	})

	t.Run("private peer with no X-Forwarded-For uses RemoteAddr", func(t *testing.T) {
		SetClientIPHeader("")
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:5555"
		if got := clientIP(req); got != "10.0.0.1" {
			t.Fatalf("want 10.0.0.1, got %q", got)
		}
	})

	t.Run("private peer with malformed right-most XFF uses RemoteAddr", func(t *testing.T) {
		SetClientIPHeader("")
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:5555"
		req.Header.Set("X-Forwarded-For", "203.0.113.7, not-an-ip")
		if got := clientIP(req); got != "10.0.0.1" {
			t.Fatalf("want 10.0.0.1, got %q", got)
		}
	})

	t.Run("RemoteAddr without a port is returned as-is", func(t *testing.T) {
		SetClientIPHeader("")
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1"
		if got := clientIP(req); got != "10.0.0.1" {
			t.Fatalf("want 10.0.0.1, got %q", got)
		}
	})
}
