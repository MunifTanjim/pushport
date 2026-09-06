package transport

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MunifTanjim/pushport/internal/app"
)

func testVAPID(t *testing.T) app.WebPushCreds {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	priv, err := key.Bytes() // 32-byte raw scalar (fixed-width, zero-padded)
	if err != nil {
		t.Fatalf("private key bytes: %v", err)
	}
	return app.WebPushCreds{
		VAPIDPrivateKey: base64.RawURLEncoding.EncodeToString(priv),
		Subject:         "mailto:ops@example.com",
	}
}

func TestWebPushSendForwardsBodyAndVAPID(t *testing.T) {
	var gotAuth, gotEnc, gotTTL string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotEnc = r.Header.Get("Content-Encoding")
		gotTTL = r.Header.Get("TTL")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)

	tr, err := NewWebPush(testVAPID(t), srv.Client())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	res, err := tr.Send(context.Background(), srv.URL+"/push/abc", Message{Ciphertext: []byte("ciphertext-bytes"), Encoding: "aes128gcm", TTL: 300, Urgency: "high"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !res.Delivered {
		t.Fatalf("expected delivered, got %+v", res)
	}
	if string(gotBody) != "ciphertext-bytes" {
		t.Fatalf("body not forwarded verbatim: %q", gotBody)
	}
	if gotEnc != "aes128gcm" || gotTTL != "300" {
		t.Fatalf("headers enc=%s ttl=%s", gotEnc, gotTTL)
	}
	if !strings.HasPrefix(gotAuth, "vapid t=") || !strings.Contains(gotAuth, ", k=") {
		t.Fatalf("vapid header malformed: %s", gotAuth)
	}
}

func TestWebPushGoneIsPermanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	t.Cleanup(srv.Close)
	tr, _ := NewWebPush(testVAPID(t), srv.Client())
	res, err := tr.Send(context.Background(), srv.URL, Message{Ciphertext: []byte("x")})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !res.Permanent {
		t.Fatalf("expected permanent, got %+v", res)
	}
}
