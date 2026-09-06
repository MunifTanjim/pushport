package db

import (
	"context"
	"testing"
)

func TestServerSettingSeedAndUpdate(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()

	s, err := d.GetServerSetting(ctx)
	if err != nil {
		t.Fatalf("get seeded: %v", err)
	}
	if s.RetryMaxAttempts != 3 || s.AuthFailIpPerMin != 5 || s.AuthFailIpBurst != 5 {
		t.Fatalf("unexpected seed defaults: %+v", s)
	}

	attempts := int64(7)
	if err := d.UpdateServerSetting(ctx, UpdateServerSettingParams{
		RetryMaxAttempts: NullInt64{Int64: attempts, Valid: true},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	s, _ = d.GetServerSetting(ctx)
	if s.RetryMaxAttempts != 7 || s.AuthFailIpPerMin != 5 {
		t.Fatalf("partial update wrong: %+v", s)
	}
}
