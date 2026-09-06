package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/MunifTanjim/pushport/internal/crypto"
	"github.com/MunifTanjim/pushport/internal/db"
)

func newSvcWithDB(t *testing.T) (*Service, *db.DB) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	return NewService(d, kr), d
}

func newSvc(t *testing.T) *Service {
	t.Helper()
	svc, _ := newSvcWithDB(t)
	return svc
}

// newSvcWithFileDB opens a file-backed DB (multi-connection pool), so concurrent
// writers actually contend — unlike :memory:, which is pinned to one connection.
func newSvcWithFileDB(t *testing.T) (*Service, *db.DB) {
	t.Helper()
	d, err := db.Open("sqlite://" + filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	return NewService(d, kr), d
}

// TestRotateEndpointKeyConcurrent guards the read-modify-write in
// RotateEndpointKey: N concurrent rotations must each bump the version by exactly
// one (no lost updates). Without the transaction, callers read the same version
// and clobber each other, leaving the final version below 1+N.
func TestRotateEndpointKeyConcurrent(t *testing.T) {
	ctx := context.Background()
	svc, d := newSvcWithFileDB(t)

	created, err := svc.Create(ctx, "acme")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.KeyVersion != 1 {
		t.Fatalf("want initial key_version 1, got %d", created.KeyVersion)
	}

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svc.RotateEndpointKey(ctx, created.ID, false); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("rotate: %v", err)
	}

	got, err := d.Queries.GetApp(ctx, created.ID)
	if err != nil {
		t.Fatalf("get app: %v", err)
	}
	if got.KeyVersion != 1+n {
		t.Fatalf("lost update: want key_version %d after %d rotations, got %d", 1+n, n, got.KeyVersion)
	}
}

func TestCreateSeedsDefaultAppUsagePlan(t *testing.T) {
	ctx := context.Background()
	svc, d := newSvcWithDB(t)

	if _, err := d.Queries.GetDefaultAppUsagePlan(ctx); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("want no default plan before first app, got err=%v", err)
	}

	if _, err := svc.Create(ctx, "app-one"); err != nil {
		t.Fatalf("create app-one: %v", err)
	}
	p, err := d.Queries.GetDefaultAppUsagePlan(ctx)
	if err != nil {
		t.Fatalf("create should seed the global default plan, got err=%v", err)
	}
	if !p.IsDefault || p.PushPerMin != 0 || p.PushDailyQuota != 0 {
		t.Fatalf("default plan should have 0/unlimited limits: %+v", p)
	}
	if p.AppID.Valid {
		t.Fatalf("global default plan must not be app-scoped: %+v", p)
	}

	if _, err := svc.Create(ctx, "app-two"); err != nil {
		t.Fatalf("create app-two: %v", err)
	}
	plans, err := d.Queries.ListAppUsagePlans(ctx)
	if err != nil {
		t.Fatalf("list plans: %v", err)
	}
	defaults := 0
	for _, pl := range plans {
		if pl.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Fatalf("want exactly one default app plan, got %d", defaults)
	}
}

func TestIsPublicAndSetPublic(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)

	tn, err := svc.Create(ctx, "myapp")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	pub, err := svc.IsPublic(ctx, tn.ID)
	if err != nil {
		t.Fatalf("IsPublic: %v", err)
	}
	if pub {
		t.Fatal("new app should not be public")
	}

	if err := svc.SetPublic(ctx, tn.ID, true); err != nil {
		t.Fatalf("SetPublic true: %v", err)
	}

	pub, err = svc.IsPublic(ctx, tn.ID)
	if err != nil {
		t.Fatalf("IsPublic after set: %v", err)
	}
	if !pub {
		t.Fatal("app should be public after SetPublic(true)")
	}

	_, err = svc.IsPublic(ctx, "nope")
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("want ErrNotFound for unknown app, got %v", err)
	}
	if err := svc.SetPublic(ctx, "nope", true); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("want ErrNotFound for SetPublic on unknown app, got %v", err)
	}
}

func TestCreateAndReadBackKey(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	tn, err := svc.Create(ctx, "myapp")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if tn.ID == "" || tn.KeyVersion != 1 {
		t.Fatalf("bad app: %+v", tn)
	}
	mkid, ver, key, err := svc.CurrentKey(ctx, tn.ID)
	if err != nil || mkid == "" || ver != 1 || len(key) != 32 {
		t.Fatalf("current key: mkid=%s ver=%d len=%d err=%v", mkid, ver, len(key), err)
	}
	byVer, err := svc.SealingKey(ctx, tn.ID, mkid, ver)
	if err != nil || len(byVer) != 32 {
		t.Fatalf("sealing key: %v", err)
	}
	if _, err := svc.SealingKey(ctx, tn.ID, "deadbeef", ver); err != ErrUnknownKID {
		t.Fatalf("want ErrUnknownKID for unknown mkid, got %v", err)
	}
	if _, err := svc.SealingKey(ctx, tn.ID, mkid, 0); err != ErrKeyRevoked {
		t.Fatalf("want ErrKeyRevoked for version below min, got %v", err)
	}
}

func TestGenerateAndAuthenticateAppToken(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)

	tn, err := svc.Create(ctx, "mgmtapp")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, ok, err := svc.AuthenticateAppToken(ctx, tn.ID, "pat_whatever")
	if err != nil {
		t.Fatalf("auth before generate: %v", err)
	}
	if ok {
		t.Fatal("should not authenticate before token is set")
	}

	raw, err := svc.GenerateAppToken(ctx, tn.ID)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.HasPrefix(raw, "pat_") {
		t.Fatalf("want pat_ prefix, got %q", raw)
	}
	if strings.Contains(raw, tn.ID) {
		t.Fatalf("token must not leak the app id: %q", raw)
	}

	got, ok, err := svc.AuthenticateAppToken(ctx, tn.ID, raw)
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	if !ok || got != tn.ID {
		t.Fatalf("correct token should authenticate to %q, got %q ok=%v", tn.ID, got, ok)
	}

	got, ok, err = svc.AuthenticateAppToken(ctx, "@app", raw)
	if err != nil {
		t.Fatalf("auth @app: %v", err)
	}
	if !ok || got != tn.ID {
		t.Fatalf("@app should resolve to %q, got %q ok=%v", tn.ID, got, ok)
	}

	_, ok, err = svc.AuthenticateAppToken(ctx, tn.ID, "pat_wrong")
	if err != nil {
		t.Fatalf("auth wrong: %v", err)
	}
	if ok {
		t.Fatal("wrong token should not authenticate")
	}

	_, ok, err = svc.AuthenticateAppToken(ctx, "nope", raw)
	if err != nil {
		t.Fatalf("auth mismatched app: %v", err)
	}
	if ok {
		t.Fatal("token must not authenticate against a mismatched app id")
	}

	raw2, err := svc.GenerateAppToken(ctx, tn.ID)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if raw2 == raw {
		t.Fatal("rotated token should differ from original")
	}

	_, ok, err = svc.AuthenticateAppToken(ctx, tn.ID, raw)
	if err != nil {
		t.Fatalf("auth old after rotate: %v", err)
	}
	if ok {
		t.Fatal("old token should not work after rotation")
	}

	_, ok, err = svc.AuthenticateAppToken(ctx, tn.ID, raw2)
	if err != nil {
		t.Fatalf("auth new: %v", err)
	}
	if !ok {
		t.Fatal("new token should authenticate")
	}

	_, err = svc.GenerateAppToken(ctx, "nope")
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("want ErrNotFound for unknown app, got %v", err)
	}
}
