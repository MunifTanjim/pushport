package app

import (
	"context"
	"errors"
	"testing"

	"github.com/MunifTanjim/pushport/internal/db"
)

func TestTurnstileRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc, id := newCredSvc(t)

	_, _, ok, err := svc.GetTurnstile(ctx, id)
	if err != nil || ok {
		t.Fatalf("expected unset, ok=%v err=%v", ok, err)
	}

	if err := svc.SetTurnstile(ctx, id, "site-key", "secret-key"); err != nil {
		t.Fatalf("set: %v", err)
	}
	site, secret, ok, err := svc.GetTurnstile(ctx, id)
	if err != nil || !ok || site != "site-key" || secret != "secret-key" {
		t.Fatalf("roundtrip: site=%q secret=%q ok=%v err=%v", site, secret, ok, err)
	}

	if err := svc.DeleteTurnstile(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, _, ok, _ = svc.GetTurnstile(ctx, id)
	if ok {
		t.Fatal("expected ok=false after delete")
	}

	if _, _, _, err := svc.GetTurnstile(ctx, "missing"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("want ErrNotFound for missing app, got %v", err)
	}
}
