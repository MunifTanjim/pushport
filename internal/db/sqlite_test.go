package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/MunifTanjim/pushport/internal/db/sqlc"
)

func newTestStore(t *testing.T) *DB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestAppCreateGet(t *testing.T) {
	ctx := context.Background()
	d := newTestStore(t)
	in := sqlc.CreateAppParams{ID: "t1", Name: "app", CreatedAt: 100}
	if err := d.CreateApp(ctx, in); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := d.GetApp(ctx, "t1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "app" || got.KeyVersion != 1 {
		t.Fatalf("unexpected app: %+v", got)
	}
	if _, err := d.GetApp(ctx, "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("want sql.ErrNoRows, got %v", err)
	}
}

func TestInstanceLookupAndRevoke(t *testing.T) {
	ctx := context.Background()
	d := newTestStore(t)
	_ = d.CreateApp(ctx, sqlc.CreateAppParams{ID: "t1", Name: "app"})
	k := sqlc.CreateInstanceParams{ID: "a1", AppID: "t1", Label: "srv", EncTokenHash: []byte{0xde, 0xad, 0xbe, 0xef}, CreatedAt: 100}
	if err := d.CreateInstance(ctx, k); err != nil {
		t.Fatalf("create instance: %v", err)
	}
	got, err := d.GetInstance(ctx, "a1")
	if err != nil || got.ID != "a1" || string(got.EncTokenHash) != string([]byte{0xde, 0xad, 0xbe, 0xef}) {
		t.Fatalf("lookup: %+v err=%v", got, err)
	}
	n, err := d.DeleteInstance(ctx, "a1")
	if err != nil || n != 1 {
		t.Fatalf("delete: n=%d err=%v", n, err)
	}
	if _, err := d.GetInstance(ctx, "a1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("want sql.ErrNoRows after hard delete, got %v", err)
	}
	if n, _ := d.DeleteInstance(ctx, "a1"); n != 0 {
		t.Fatalf("want 0 rows deleting missing instance, got %d", n)
	}
}

func TestAppCredentialTable(t *testing.T) {
	ctx := context.Background()
	d := newTestStore(t)
	_ = d.CreateApp(ctx, sqlc.CreateAppParams{ID: "t1", Name: "app"})

	if err := d.UpsertAppCredential(ctx, sqlc.UpsertAppCredentialParams{AppID: "t1", Transport: "apns", EncBlob: []byte{9, 9, 9}}); err != nil {
		t.Fatalf("upsert apns: %v", err)
	}
	if err := d.UpsertAppCredential(ctx, sqlc.UpsertAppCredentialParams{AppID: "t1", Transport: "fcm", EncBlob: []byte{1, 2}}); err != nil {
		t.Fatalf("upsert fcm: %v", err)
	}
	rows, _ := d.ListAppCredentials(ctx, "t1")
	sizes := map[string]int{}
	for _, r := range rows {
		sizes[r.Transport] = len(r.EncBlob)
	}
	if len(rows) != 2 || sizes["apns"] != 3 || sizes["fcm"] != 2 {
		t.Fatalf("creds rows wrong: %+v (rows=%d)", sizes, len(rows))
	}

	if err := d.UpsertAppCredential(ctx, sqlc.UpsertAppCredentialParams{AppID: "t1", Transport: "apns", EncBlob: []byte{7}}); err != nil {
		t.Fatalf("re-upsert apns: %v", err)
	}
	rows, _ = d.ListAppCredentials(ctx, "t1")
	if len(rows) != 2 {
		t.Fatalf("upsert should replace, not duplicate: rows=%d", len(rows))
	}

	if err := d.DeleteAppCredential(ctx, sqlc.DeleteAppCredentialParams{AppID: "t1", Transport: "apns"}); err != nil {
		t.Fatalf("delete apns: %v", err)
	}
	rows, _ = d.ListAppCredentials(ctx, "t1")
	if len(rows) != 1 || rows[0].Transport != "fcm" {
		t.Fatalf("after delete apns: %+v", rows)
	}

	if err := d.UpsertAppCredential(ctx, sqlc.UpsertAppCredentialParams{AppID: "missing", Transport: "apns", EncBlob: []byte{1}}); err == nil {
		t.Fatal("expected FK violation for unknown app_id")
	}
}

func TestSetAndClearTurnstile(t *testing.T) {
	ctx := context.Background()
	d := newTestStore(t)
	_ = d.CreateApp(ctx, sqlc.CreateAppParams{ID: "t1", Name: "app"})

	n, err := d.SetAppTurnstile(ctx, sqlc.SetAppTurnstileParams{
		TurnstileSiteKey:   NewNullString("site"),
		EncTurnstileSecret: []byte{1, 2, 3},
		ID:                 "t1",
	})
	if err != nil || n != 1 {
		t.Fatalf("set: n=%d err=%v", n, err)
	}
	got, _ := d.GetApp(ctx, "t1")
	if got.TurnstileSiteKey.String != "site" || len(got.EncTurnstileSecret) != 3 {
		t.Fatalf("not stored: %+v", got)
	}

	if _, err := d.ClearAppTurnstile(ctx, "t1"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	got, _ = d.GetApp(ctx, "t1")
	if got.TurnstileSiteKey.Valid || len(got.EncTurnstileSecret) != 0 {
		t.Fatalf("not cleared: %+v", got)
	}

	if n, _ := d.SetAppTurnstile(ctx, sqlc.SetAppTurnstileParams{ID: "missing"}); n != 0 {
		t.Fatalf("want 0 rows for missing app, got %d", n)
	}
}
