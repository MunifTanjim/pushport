package instance

import (
	"context"
	"strings"
	"testing"

	"github.com/MunifTanjim/pushport/internal/crypto"
	"github.com/MunifTanjim/pushport/internal/db"
)

func newInstSvc(t *testing.T) (*Service, *db.DB) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	_ = d.CreateApp(context.Background(), db.CreateAppParams{ID: "t1", Name: "app"})
	kr, _ := crypto.NewKeyring([][]byte{make([]byte, 32)})
	return NewService(d.Queries, kr), d
}

func TestIssueAuthenticateRevoke(t *testing.T) {
	ctx := context.Background()
	svc, _ := newInstSvc(t)
	raw, rec, err := svc.Issue(ctx, "t1", "server-a")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !strings.HasPrefix(raw, "pit_") {
		t.Fatalf("bad raw token: %s", raw)
	}
	if strings.Contains(raw, rec.ID) {
		t.Fatalf("token must not leak the instance id: %s", raw)
	}
	got, err := svc.Authenticate(ctx, raw)
	if err != nil || got.AppID != "t1" {
		t.Fatalf("authenticate: %+v err=%v", got, err)
	}
	if err := svc.Delete(ctx, rec.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := svc.Authenticate(ctx, raw); err != ErrInvalid {
		t.Fatalf("want ErrInvalid after revoke, got %v", err)
	}
	if _, err := svc.Authenticate(ctx, "pit_bogus"); err != ErrInvalid {
		t.Fatalf("want ErrInvalid for bogus, got %v", err)
	}
}

func TestInstanceTokenRoundTrip(t *testing.T) {
	svc, _ := newInstSvc(t)
	ctx := context.Background()
	raw, rec, err := svc.Issue(ctx, "t1", "srv")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !strings.HasPrefix(raw, "pit_") {
		t.Fatalf("bad token format: %q", raw)
	}
	if strings.Contains(raw, rec.ID) {
		t.Fatalf("token must not leak the instance id: %q", raw)
	}
	got, err := svc.Authenticate(ctx, raw)
	if err != nil || got.ID != rec.ID {
		t.Fatalf("auth: rec=%+v err=%v", got, err)
	}
	rotated, err := svc.RotateToken(ctx, rec.ID)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := svc.Authenticate(ctx, raw); err != ErrInvalid {
		t.Fatalf("old token should fail after rotate, got %v", err)
	}
	if got, err := svc.Authenticate(ctx, rotated); err != nil || got.ID != rec.ID {
		t.Fatalf("rotated token should auth: rec=%+v err=%v", got, err)
	}
	if _, err := svc.Authenticate(ctx, "garbage"); err == nil {
		t.Fatal("malformed should fail")
	}
}
