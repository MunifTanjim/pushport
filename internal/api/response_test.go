package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serve runs h behind the request-id middleware and returns the recorder.
func serve(t *testing.T, method, target, body, contentType string, h http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	WithRequestID(h).ServeHTTP(rec, r)
	return rec
}

func TestSendDataEnvelope(t *testing.T) {
	rec := serve(t, "GET", "/x", "", "", func(w http.ResponseWriter, r *http.Request) {
		SendData(w, r, http.StatusCreated, map[string]string{"token": "pit_abc"})
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d", rec.Code)
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatal("missing X-Request-Id header")
	}
	var out struct {
		RequestID string            `json:"request_id"`
		Data      map[string]string `json:"data"`
		Error     any               `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	if out.RequestID == "" || out.RequestID != rec.Header().Get("X-Request-Id") {
		t.Fatalf("request_id mismatch: body=%q header=%q", out.RequestID, rec.Header().Get("X-Request-Id"))
	}
	if out.Data["token"] != "pit_abc" || out.Error != nil {
		t.Fatalf("bad envelope: %s", rec.Body.String())
	}
}

func TestSendDataNoContent(t *testing.T) {
	rec := serve(t, "DELETE", "/x", "", "", func(w http.ResponseWriter, r *http.Request) {
		SendData(w, r, http.StatusNoContent, nil)
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("204 should have empty body, got %q", rec.Body.String())
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatal("204 should still carry X-Request-Id header")
	}
}

func TestSendErrorAPIError(t *testing.T) {
	rec := serve(t, "GET", "/apps/x", "", "", func(w http.ResponseWriter, r *http.Request) {
		SendError(w, r, ErrorNotFound().WithMessage("app not found"))
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: %d", rec.Code)
	}
	var out struct {
		RequestID string `json:"request_id"`
		Error     struct {
			Code       string `json:"code"`
			Message    string `json:"message"`
			StatusCode int    `json:"status_code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Error.Code != "not_found" || out.Error.Message != "app not found" || out.Error.StatusCode != 404 {
		t.Fatalf("bad error envelope: %s", rec.Body.String())
	}
	if out.RequestID == "" {
		t.Fatal("error envelope missing request_id")
	}
	if strings.Contains(rec.Body.String(), "\"method\"") || strings.Contains(rec.Body.String(), "\"path\"") || strings.Contains(rec.Body.String(), "\"cause\"") {
		t.Fatalf("error body leaked method/path/cause: %s", rec.Body.String())
	}
}

func TestSendErrorPlainErrorBecomes500(t *testing.T) {
	rec := serve(t, "GET", "/x", "", "", func(w http.ResponseWriter, r *http.Request) {
		SendError(w, r, errors.New("boom"))
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "internal_server_error") {
		t.Fatalf("want internal_server_error, got %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("cause must not be serialized: %s", rec.Body.String())
	}
}

func TestSendErrorRetryAfter(t *testing.T) {
	rec := serve(t, "GET", "/x", "", "", func(w http.ResponseWriter, r *http.Request) {
		SendError(w, r, ErrorTooManyRequests().WithRetryAfter("42"))
	})
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "42" {
		t.Fatalf("status=%d retry-after=%q", rec.Code, rec.Header().Get("Retry-After"))
	}
}

func TestReadRequestBodyJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	r := httptest.NewRequest("POST", "/x", strings.NewReader(`{"name":"a"}`))
	var p payload
	if err := ReadRequestBodyJSON(httptest.NewRecorder(), r, &p); err == nil {
		t.Fatal("want error for missing content type")
	} else if e, ok := err.(*APIError); !ok || e.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415 APIError, got %v", err)
	}

	r = httptest.NewRequest("POST", "/x", strings.NewReader(`{"name":"a"}`))
	r.Header.Set("Content-Type", "application/json")
	p = payload{}
	if err := ReadRequestBodyJSON(httptest.NewRecorder(), r, &p); err != nil {
		t.Fatalf("valid decode: %v", err)
	}
	if p.Name != "a" {
		t.Fatalf("decoded %+v", p)
	}

	r = httptest.NewRequest("POST", "/x", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/json")
	if err := ReadRequestBodyJSON(httptest.NewRecorder(), r, &p); err == nil {
		t.Fatal("want error for empty body")
	} else if e, ok := err.(*APIError); !ok || e.StatusCode != http.StatusBadRequest || e.Message != "missing body" {
		t.Fatalf("want 400 missing body, got %v", err)
	}
}

func TestReadRequestBodyJSONTooLarge(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", strings.NewReader(`{"name":"aaaa"}`))
	r.Header.Set("Content-Type", "application/json")
	var p struct {
		Name string `json:"name"`
	}
	err := ReadRequestBodyJSON(httptest.NewRecorder(), r, &p, 8)
	var e *APIError
	if !errors.As(err, &e) || e.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413 APIError, got %v", err)
	}
}
