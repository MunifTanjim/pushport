package transport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func staticTS() oauth2.TokenSource {
	return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-access-token"})
}

// TestFCMTokenSourceHonorsClientTimeout: the OAuth token exchange must run on the
// injected client (oauth2.HTTPClient) so its 200ms timeout applies — on the
// untimed http.DefaultClient the fetch against a stalling endpoint would hang.
func TestFCMTokenSourceHonorsClientTimeout(t *testing.T) {
	stall := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second): // safety net so Close never hangs
		}
	}))
	defer stall.Close()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	saJSON, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     "proj",
		"private_key_id": "kid",
		"private_key":    string(pemBytes),
		"client_email":   "svc@proj.iam.gserviceaccount.com",
		"token_uri":      stall.URL,
	})
	if err != nil {
		t.Fatalf("marshal service account: %v", err)
	}

	ts, err := FCMTokenSourceFromJSON(string(saJSON), &http.Client{Timeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatalf("token source: %v", err)
	}

	start := time.Now()
	if _, err := ts.Token(); err == nil {
		t.Fatal("expected token fetch to fail against a stalling endpoint")
	}
	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
		t.Fatalf("token fetch took %v; injected client timeout (200ms) not applied", elapsed)
	}
}

func TestFCMSendSuccess(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"projects/p/messages/1"}`))
	}))
	t.Cleanup(srv.Close)

	tr := NewFCM("proj-123", staticTS(), srv.URL, srv.Client())
	res, err := tr.Send(context.Background(), "fcmtoken", Message{Ciphertext: []byte("secret"), Encoding: "aes128gcm", TTL: 120, Urgency: "high"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !res.Delivered {
		t.Fatalf("expected delivered, got %+v", res)
	}
	if gotPath != "/v1/projects/proj-123/messages:send" || gotAuth != "Bearer test-access-token" {
		t.Fatalf("path=%s auth=%s", gotPath, gotAuth)
	}
	message, _ := gotBody["message"].(map[string]any)
	data, _ := message["data"].(map[string]any)
	if data == nil || data["e"] == nil {
		t.Fatalf("data.e missing: %v", gotBody)
	}
}

func TestFCMNotFoundIsPermanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"status":"NOT_FOUND"}}`))
	}))
	t.Cleanup(srv.Close)
	tr := NewFCM("p", staticTS(), srv.URL, srv.Client())
	res, err := tr.Send(context.Background(), "dead", Message{Ciphertext: []byte("x")})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !res.Permanent || res.Delivered {
		t.Fatalf("expected permanent, got %+v", res)
	}
}
