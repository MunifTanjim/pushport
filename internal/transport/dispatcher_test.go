package transport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MunifTanjim/pushport/internal/app"
)

type fakeCreds struct{ c app.Credentials }

func (f fakeCreds) GetCredentials(_ context.Context, _ string) (app.Credentials, error) {
	return f.c, nil
}

func TestDispatcherRoutesToWebPush(t *testing.T) {
	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)

	d := NewDispatcher(fakeCreds{c: app.Credentials{WebPush: ptrVAPID(t)}}, srv.Client())
	res, err := d.Send(context.Background(), "t1", "webpush", srv.URL+"/sub", SendOptions{}, Message{Ciphertext: []byte("x"), TTL: 60})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !res.Delivered || !hit {
		t.Fatalf("expected delivered+hit, got %+v hit=%v", res, hit)
	}
}

func TestDispatcherNotConfigured(t *testing.T) {
	d := NewDispatcher(fakeCreds{c: app.Credentials{}}, http.DefaultClient)
	_, err := d.Send(context.Background(), "t1", "apns", "devtoken", SendOptions{}, Message{Ciphertext: []byte("x")})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func ptrVAPID(t *testing.T) *app.WebPushCreds {
	v := testVAPID(t)
	return &v
}

func TestDispatcherInvalidCredentials(t *testing.T) {
	bad := app.Credentials{APNs: &app.APNsCreds{KeyP8: "not-a-pem", KeyID: "K", TeamID: "T", Topic: "com.x"}}
	d := NewDispatcher(fakeCreds{c: bad}, http.DefaultClient)
	_, err := d.Send(context.Background(), "t1", "apns", "devtoken", SendOptions{}, Message{Ciphertext: []byte("x")})
	if !errors.Is(err, ErrCredentialsInvalid) {
		t.Fatalf("want ErrCredentialsInvalid, got %v", err)
	}
}

func TestDispatcherAPNsSandboxRouting(t *testing.T) {
	d := NewDispatcher(fakeCreds{c: app.Credentials{APNs: &app.APNsCreds{KeyP8: testP8(t), KeyID: "K1", TeamID: "T1", Topic: "com.x"}}}, http.DefaultClient)

	b, err := d.bundleFor(context.Background(), "t1")
	if err != nil {
		t.Fatalf("bundleFor: %v", err)
	}
	if b.apns == nil || b.apnsSandbox == nil {
		t.Fatalf("expected both apns clients built, got %+v", b)
	}
	if b.apns == b.apnsSandbox {
		t.Fatal("prod and sandbox should be distinct clients")
	}
	prod, ok := b.apns.(*APNs)
	if !ok || prod.baseURL != "https://api.push.apple.com" {
		t.Fatalf("prod client host wrong: %+v", b.apns)
	}
	sb, ok := b.apnsSandbox.(*APNs)
	if !ok || sb.baseURL != "https://api.sandbox.push.apple.com" {
		t.Fatalf("sandbox client host wrong: %+v", b.apnsSandbox)
	}
}
