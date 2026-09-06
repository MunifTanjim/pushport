package seal

import (
	"context"
	"crypto/rand"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MunifTanjim/pushport/internal/app"
	"github.com/MunifTanjim/pushport/internal/crypto"
	"github.com/MunifTanjim/pushport/internal/db"
)

var ctx = context.Background()

func newSealFixture(t *testing.T) (*Service, string) {
	t.Helper()
	svc, _, appID := newSealFixtureWithDB(t)
	return svc, appID
}

func newSealFixtureWithDB(t *testing.T) (*Service, *db.DB, string) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	appSvc := app.NewService(d, kr)
	a, err := appSvc.Create(ctx, "app")
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	return NewService(appSvc), d, a.ID
}

func TestSealOpenRoundTrip(t *testing.T) {
	svc, appID := newSealFixture(t)
	tok, err := svc.Seal(ctx, appID, Payload{Transport: "apns", TransportRef: "ref", Exp: time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(tok, ".") != 3 {
		t.Fatalf("want 4 segments, got %q", tok)
	}
	gotApp, p, err := svc.Open(ctx, tok)
	if err != nil || gotApp != appID || p.TransportRef != "ref" {
		t.Fatalf("open: %s %+v %v", gotApp, p, err)
	}
}

func TestSealRejectsNonPositiveExpiry(t *testing.T) {
	svc, appID := newSealFixture(t)
	for _, exp := range []int64{0, -1} {
		if _, err := svc.Seal(ctx, appID, Payload{Transport: "apns", TransportRef: "r", Exp: exp}); err == nil {
			t.Fatalf("Seal should reject Exp=%d (never-expiring token)", exp)
		}
	}
}

func TestOpenRejectsRevokedVersion(t *testing.T) {
	svc, d, appID := newSealFixtureWithDB(t)
	tok, _ := svc.Seal(ctx, appID, Payload{Transport: "apns", TransportRef: "r", Exp: time.Now().Add(time.Hour).Unix()})
	if _, err := d.Queries.UpdateAppKeyVersion(ctx, db.UpdateAppKeyVersionParams{
		KeyVersion:    1,
		MinKeyVersion: 2,
		ID:            appID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Open(ctx, tok); !errors.Is(err, ErrExpired) {
		t.Fatalf("want ErrExpired for revoked version, got %v", err)
	}
}

func TestOpenRejectsUnknownMkid(t *testing.T) {
	svc, appID := newSealFixture(t)
	tok, err := svc.Seal(ctx, appID, Payload{Transport: "apns", TransportRef: "r", Exp: time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(tok, ".", 4)
	ver := parts[2]
	verInt, _ := strconv.Atoi(ver)
	fabricated := appID + ".deadbeef." + strconv.Itoa(verInt) + "." + parts[3]
	if _, _, err := svc.Open(ctx, fabricated); !errors.Is(err, ErrExpired) {
		t.Fatalf("want ErrExpired for unknown mkid, got %v", err)
	}
}

func TestOpenRejectsExpired(t *testing.T) {
	svc, tid := newSealFixture(t)
	p := Payload{Transport: "fcm", TransportRef: "x", Exp: time.Now().Add(-time.Minute).Unix(), JTI: "j2"}
	token, _ := svc.Seal(ctx, tid, p)
	if _, _, err := svc.Open(ctx, token); err != ErrExpired {
		t.Fatalf("want ErrExpired, got %v", err)
	}
}

func TestOpenRejectsMalformed(t *testing.T) {
	svc, _ := newSealFixture(t)
	if _, _, err := svc.Open(ctx, "not-a-token"); err != ErrMalformed {
		t.Fatalf("want ErrMalformed, got %v", err)
	}
}

func TestOpenRejectsTamperedBlob(t *testing.T) {
	svc, appID := newSealFixture(t)
	tok, err := svc.Seal(ctx, appID, Payload{Transport: "apns", TransportRef: "r", Exp: time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(tok, ".", 4)
	corrupted := parts[0] + "." + parts[1] + "." + parts[2] + "." + parts[3][:len(parts[3])/2]
	if _, _, err := svc.Open(ctx, corrupted); !errors.Is(err, ErrMalformed) {
		t.Fatalf("want ErrMalformed for tampered blob, got %v", err)
	}
}
