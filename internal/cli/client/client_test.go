package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDoSuccessDecodesData(t *testing.T) {
	var gotAuth, gotCT, gotBody, gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"request_id":"r1","data":{"id":"a1"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok", 5*time.Second)
	data, err := c.Do(context.Background(), "POST", "/apps", map[string]string{"name": "x"})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAuth != "Bearer tok" || gotCT != "application/json" {
		t.Fatalf("headers: auth=%q ct=%q", gotAuth, gotCT)
	}
	if gotMethod != "POST" || gotPath != "/apps" || gotBody != `{"name":"x"}` {
		t.Fatalf("req: %s %s body=%s", gotMethod, gotPath, gotBody)
	}
	var got struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &got); err != nil || got.ID != "a1" {
		t.Fatalf("data: %v %+v", err, got)
	}
}

func TestDoErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"request_id":"r1","error":{"code":"not_found","message":"app not found","status_code":404}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok", 5*time.Second)
	_, err := c.Do(context.Background(), "GET", "/apps/x", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 || apiErr.Message != "app not found" {
		t.Fatalf("want APIError 404, got %v", err)
	}
}

func TestDoErrorEnvelopeOn2xx(t *testing.T) {
	// A 200 that nonetheless carries an error envelope is treated as an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"request_id":"r","error":{"code":"conflict","message":"nope","status_code":409}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok", 5*time.Second)
	_, err := c.Do(context.Background(), "GET", "/x", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Message != "nope" {
		t.Fatalf("want APIError from error envelope on 2xx, got %v", err)
	}
}

func TestDoNoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			_, _ = io.ReadAll(r.Body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "tok", 5*time.Second)
	data, err := c.Do(context.Background(), "DELETE", "/apps/x/creds/apns", nil)
	if err != nil || data != nil {
		t.Fatalf("want nil,nil got data=%s err=%v", data, err)
	}
}
