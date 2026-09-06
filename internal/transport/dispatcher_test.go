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
	res, err := d.Send(context.Background(), "t1", "webpush", srv.URL+"/sub", Message{Ciphertext: []byte("x"), TTL: 60})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !res.Delivered || !hit {
		t.Fatalf("expected delivered+hit, got %+v hit=%v", res, hit)
	}
}

func TestDispatcherNotConfigured(t *testing.T) {
	d := NewDispatcher(fakeCreds{c: app.Credentials{}}, http.DefaultClient)
	_, err := d.Send(context.Background(), "t1", "apns", "devtoken", Message{Ciphertext: []byte("x")})
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
	_, err := d.Send(context.Background(), "t1", "apns", "devtoken", Message{Ciphertext: []byte("x")})
	if !errors.Is(err, ErrCredentialsInvalid) {
		t.Fatalf("want ErrCredentialsInvalid, got %v", err)
	}
}
