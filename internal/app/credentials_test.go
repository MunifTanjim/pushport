package app

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/MunifTanjim/pushport/internal/crypto"
	"github.com/MunifTanjim/pushport/internal/db"
)

func newCredSvc(t *testing.T) (*Service, string) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	svc := NewService(d, kr)
	tn, err := svc.Create(context.Background(), "app")
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	return svc, tn.ID
}

func TestCredentialsRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc, tid := newCredSvc(t)

	got, err := svc.GetCredentials(ctx, tid)
	if err != nil {
		t.Fatalf("get empty: %v", err)
	}
	if got.APNs != nil || got.FCM != nil || got.WebPush != nil {
		t.Fatalf("expected empty credentials, got %+v", got)
	}

	if err := svc.SetAPNsCredentials(ctx, tid, APNsCreds{KeyP8: "PEM", KeyID: "K1", TeamID: "T1", Topic: "com.example.app", Production: true}); err != nil {
		t.Fatalf("set apns: %v", err)
	}
	if err := svc.SetWebPushCredentials(ctx, tid, WebPushCreds{VAPIDPrivateKey: "priv", Subject: "mailto:a@b.c"}); err != nil {
		t.Fatalf("set webpush: %v", err)
	}
	got, err = svc.GetCredentials(ctx, tid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.APNs == nil || got.APNs.KeyID != "K1" || !got.APNs.Production {
		t.Fatalf("apns not round-tripped: %+v", got.APNs)
	}
	if got.WebPush == nil || got.WebPush.Subject != "mailto:a@b.c" {
		t.Fatalf("webpush not round-tripped: %+v", got.WebPush)
	}
	if got.FCM != nil {
		t.Fatalf("fcm should be nil: %+v", got.FCM)
	}

	if err := svc.DeleteCredentials(ctx, tid, "apns"); err != nil {
		t.Fatalf("delete apns: %v", err)
	}
	got, err = svc.GetCredentials(ctx, tid)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if got.APNs != nil {
		t.Fatalf("apns should be cleared: %+v", got.APNs)
	}
	if got.WebPush == nil {
		t.Fatalf("webpush should remain after deleting apns")
	}
}

func TestFCMProjectIDExtract(t *testing.T) {
	id, err := FCMProjectID(`{"type":"service_account","project_id":"p1"}`)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if id != "p1" {
		t.Fatalf("got %q, want p1", id)
	}
	if _, err := FCMProjectID(`{"type":"service_account"}`); err == nil {
		t.Fatal("expected error for missing project_id")
	}
	if _, err := FCMProjectID("not json"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSetFCMCredentialsValidatesJSON(t *testing.T) {
	ctx := context.Background()
	svc, tid := newCredSvc(t)

	if err := svc.SetFCMCredentials(ctx, tid, FCMCreds{ServiceAccountJSON: "not json"}); !errors.Is(err, ErrInvalidCreds) {
		t.Fatalf("invalid json: want ErrInvalidCreds, got %v", err)
	}
	if err := svc.SetFCMCredentials(ctx, tid, FCMCreds{ServiceAccountJSON: `{"type":"service_account"}`}); !errors.Is(err, ErrInvalidCreds) {
		t.Fatalf("missing project_id: want ErrInvalidCreds, got %v", err)
	}

	sa := `{"type":"service_account","project_id":"proj"}`
	if err := svc.SetFCMCredentials(ctx, tid, FCMCreds{ServiceAccountJSON: sa}); err != nil {
		t.Fatalf("set fcm: %v", err)
	}
	got, err := svc.GetCredentials(ctx, tid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.FCM == nil || got.FCM.ServiceAccountJSON != sa {
		t.Fatalf("fcm not round-tripped: %+v", got.FCM)
	}
}
