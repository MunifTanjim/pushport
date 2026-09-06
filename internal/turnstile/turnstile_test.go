package turnstile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func stubServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("secret") == "" || r.FormValue("response") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestVerifySuccess(t *testing.T) {
	srv := stubServer(t, `{"success":true}`)
	c := New(nil)
	c.url = srv.URL
	ok, err := c.Verify(context.Background(), "sec", "tok", "1.2.3.4")
	if err != nil || !ok {
		t.Fatalf("want success, ok=%v err=%v", ok, err)
	}
}

func TestVerifyFailure(t *testing.T) {
	srv := stubServer(t, `{"success":false,"error-codes":["invalid-input-response"]}`)
	c := New(nil)
	c.url = srv.URL
	ok, err := c.Verify(context.Background(), "sec", "tok", "")
	if err != nil || ok {
		t.Fatalf("want ok=false no err, ok=%v err=%v", ok, err)
	}
}

func TestVerifyTransportError(t *testing.T) {
	c := New(nil)
	c.url = "http://127.0.0.1:1" // nothing listening
	if _, err := c.Verify(context.Background(), "sec", "tok", ""); err == nil {
		t.Fatal("want transport error")
	}
}
