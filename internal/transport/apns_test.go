package transport

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MunifTanjim/pushport/internal/app"
)

func testP8(t *testing.T) string {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestAPNsSendSuccess(t *testing.T) {
	var gotPath, gotAuth, gotTopic, gotPushType string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("authorization")
		gotTopic = r.Header.Get("apns-topic")
		gotPushType = r.Header.Get("apns-push-type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	tr, err := NewAPNs(app.APNsCreds{KeyP8: testP8(t), KeyID: "K1", TeamID: "T1", Topic: "com.example.app"}, srv.URL, false, srv.Client())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	res, err := tr.Send(context.Background(), "devtoken123", Message{Ciphertext: []byte("secret"), Encoding: "aes128gcm", TTL: 60, Urgency: "high"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !res.Delivered {
		t.Fatalf("expected delivered, got %+v", res)
	}
	if gotPath != "/3/device/devtoken123" {
		t.Fatalf("path=%s", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "bearer ") {
		t.Fatalf("auth=%s", gotAuth)
	}
	if gotTopic != "com.example.app" || gotPushType != "alert" {
		t.Fatalf("headers topic=%s pushtype=%s", gotTopic, gotPushType)
	}
	aps, _ := gotBody["aps"].(map[string]any)
	if aps == nil || aps["mutable-content"] == nil {
		t.Fatalf("aps missing mutable-content: %v", gotBody)
	}
	if gotBody["e"] == nil {
		t.Fatalf("ciphertext field missing: %v", gotBody)
	}
}

func TestAPNsGoneIsPermanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"reason":"Unregistered"}`))
	}))
	t.Cleanup(srv.Close)
	tr, _ := NewAPNs(app.APNsCreds{KeyP8: testP8(t), KeyID: "K1", TeamID: "T1", Topic: "com.x"}, srv.URL, false, srv.Client())
	res, err := tr.Send(context.Background(), "dead", Message{Ciphertext: []byte("x")})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if res.Delivered || !res.Permanent || res.StatusCode != http.StatusGone {
		t.Fatalf("expected permanent gone, got %+v", res)
	}
}

func TestRetryAfterParsesSecondsAndDate(t *testing.T) {
	mk := func(v string) *http.Response {
		return &http.Response{Header: http.Header{"Retry-After": []string{v}}}
	}
	if got := retryAfter(mk("120")); got != 120*time.Second {
		t.Fatalf("seconds: want 120s, got %v", got)
	}
	if got := retryAfter(&http.Response{Header: http.Header{}}); got != 0 {
		t.Fatalf("absent: want 0, got %v", got)
	}
	if got := retryAfter(mk("-5")); got != 0 {
		t.Fatalf("negative: want 0, got %v", got)
	}
	if got := retryAfter(mk("garbage")); got != 0 {
		t.Fatalf("garbage: want 0, got %v", got)
	}
	future := time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)
	if got := retryAfter(mk(future)); got <= 0 || got > 61*time.Minute {
		t.Fatalf("http-date future: want ~1h, got %v", got)
	}
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	if got := retryAfter(mk(past)); got != 0 {
		t.Fatalf("http-date past: want 0, got %v", got)
	}
}

func TestAPNsExpiredProviderTokenRetriesAndInvalidatesJWT(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"reason":"ExpiredProviderToken"}`))
	}))
	t.Cleanup(srv.Close)
	tr, _ := NewAPNs(app.APNsCreds{KeyP8: testP8(t), KeyID: "K1", TeamID: "T1", Topic: "com.x"}, srv.URL, false, srv.Client())
	res, err := tr.Send(context.Background(), "dev", Message{Ciphertext: []byte("x")})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if res.StatusCode != http.StatusForbidden || !res.Retryable {
		t.Fatalf("want retryable 403, got %+v", res)
	}
	if !transient(res, nil) {
		t.Fatal("expired provider token should be transient (retried)")
	}
	if tr.jwt != "" {
		t.Fatal("cached JWT should be invalidated after 403 ExpiredProviderToken")
	}
}

func TestAPNsInvalidProviderTokenNotRetryableButInvalidatesJWT(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"reason":"InvalidProviderToken"}`))
	}))
	t.Cleanup(srv.Close)
	tr, _ := NewAPNs(app.APNsCreds{KeyP8: testP8(t), KeyID: "K1", TeamID: "T1", Topic: "com.x"}, srv.URL, false, srv.Client())
	res, _ := tr.Send(context.Background(), "dev", Message{Ciphertext: []byte("x")})
	if res.StatusCode != http.StatusForbidden || res.Retryable {
		t.Fatalf("invalid provider token should not be retryable, got %+v", res)
	}
	if transient(res, nil) {
		t.Fatal("invalid provider token should not be transient")
	}
	if tr.jwt != "" {
		t.Fatal("cached JWT should still be invalidated after 403 InvalidProviderToken")
	}
}

func TestAPNsBadDeviceTokenIsPermanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"reason":"BadDeviceToken"}`))
	}))
	t.Cleanup(srv.Close)
	tr, _ := NewAPNs(app.APNsCreds{KeyP8: testP8(t), KeyID: "K1", TeamID: "T1", Topic: "com.x"}, srv.URL, false, srv.Client())
	res, err := tr.Send(context.Background(), "badtoken", Message{Ciphertext: []byte("x")})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !res.Permanent || res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected permanent 400, got %+v", res)
	}
}

func TestAPNsSandboxSelectsEnvironment(t *testing.T) {
	// Both hosts map to the test server via baseURL override; the point is that
	// the sandbox flag still picks the default hosts when baseURL is empty.
	prod, err := NewAPNs(app.APNsCreds{KeyP8: testP8(t), KeyID: "K1", TeamID: "T1", Topic: "com.x"}, "", false, http.DefaultClient)
	if err != nil {
		t.Fatalf("prod: %v", err)
	}
	sandbox, err := NewAPNs(app.APNsCreds{KeyP8: testP8(t), KeyID: "K1", TeamID: "T1", Topic: "com.x"}, "", true, http.DefaultClient)
	if err != nil {
		t.Fatalf("sandbox: %v", err)
	}
	if prod.baseURL != "https://api.push.apple.com" {
		t.Fatalf("prod host: %s", prod.baseURL)
	}
	if sandbox.baseURL != "https://api.sandbox.push.apple.com" {
		t.Fatalf("sandbox host: %s", sandbox.baseURL)
	}
}
