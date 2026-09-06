package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/MunifTanjim/pushport/internal/crypto"
	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/id"
)

var (
	ErrUnknownKID = errors.New("app: unknown kid")
	ErrKeyRevoked = errors.New("app: endpoint key revoked")
)

type Service struct {
	db      *db.DB
	q       *db.Queries
	keyring *crypto.Keyring
}

func NewService(dbh *db.DB, kr *crypto.Keyring) *Service {
	return &Service{db: dbh, q: dbh.Queries, keyring: kr}
}

func (svc *Service) Create(ctx context.Context, name string) (db.App, error) {
	appID := id.New()
	now := time.Now().Unix()
	// All three writes must land together: an app with no default instance plan
	// (or missing global default) is permanently broken for quota resolution.
	err := svc.db.InTx(ctx, func(q *db.Queries) error {
		if err := q.CreateApp(ctx, db.CreateAppParams{
			ID:        appID,
			Name:      name,
			CreatedAt: now,
		}); err != nil {
			return err
		}
		// The global default app plan is the fallback the resolver uses for
		// any app with no assigned plan.
		if err := ensureDefaultAppUsagePlan(ctx, q, now); err != nil {
			return err
		}
		// The undeletable default instance plan is the fallback the
		// resolver uses for any instance with no assigned plan.
		return q.CreateDefaultInstanceUsagePlan(ctx, db.CreateDefaultInstanceUsagePlanParams{
			ID:        id.New(),
			AppID:     appID,
			Name:      "default",
			CreatedAt: now,
		})
	})
	if err != nil {
		return db.App{}, err
	}
	return db.App{ID: appID, Name: name, KeyVersion: 1, MinKeyVersion: 1, CreatedAt: now}, nil
}

// ensureDefaultAppUsagePlan creates the single global default app_usage_plan row
// if it does not exist. It is idempotent: an existing default is left untouched,
// and a concurrent create that loses the unique-index race is treated as success.
func ensureDefaultAppUsagePlan(ctx context.Context, q *db.Queries, now int64) error {
	switch _, err := q.GetDefaultAppUsagePlan(ctx); {
	case err == nil:
		return nil
	case !errors.Is(err, sql.ErrNoRows):
		return err
	}
	err := q.CreateDefaultAppUsagePlan(ctx, db.CreateDefaultAppUsagePlanParams{
		ID:        id.New(),
		Name:      "default",
		CreatedAt: now,
	})
	if err != nil && !db.IsUniqueViolation(err) {
		return err
	}
	return nil
}

func (svc *Service) IsPublic(ctx context.Context, appID string) (bool, error) {
	tn, err := svc.q.GetApp(ctx, appID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, db.ErrNotFound
	}
	if err != nil {
		return false, err
	}
	return tn.IsPublic, nil
}

func (svc *Service) SetPublic(ctx context.Context, appID string, public bool) error {
	n, err := svc.q.SetAppPublic(ctx, db.SetAppPublicParams{ID: appID, IsPublic: public})
	if err != nil {
		return err
	}
	if n == 0 {
		return db.ErrNotFound
	}
	return nil
}

// Update partially updates an app's mutable fields. nil fields are left unchanged.
func (svc *Service) Update(ctx context.Context, appID string, name *string, public *bool) error {
	params := db.UpdateAppParams{ID: appID}
	if name != nil {
		params.Name = db.NewNullString(name)
	}
	if public != nil {
		params.IsPublic = sql.NullBool{Bool: *public, Valid: true}
	}
	n, err := svc.q.UpdateApp(ctx, params)
	if err != nil {
		return err
	}
	if n == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (svc *Service) CurrentKey(ctx context.Context, appID string) (mkid string, keyVersion int, key []byte, err error) {
	tn, err := svc.q.GetApp(ctx, appID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil, db.ErrNotFound
	}
	if err != nil {
		return "", 0, nil, err
	}
	mkid = svc.keyring.PrimaryMkid()
	ver := int(tn.KeyVersion)
	k, ok := svc.keyring.DeriveEndpointKey(mkid, appID, ver)
	if !ok {
		return "", 0, nil, ErrUnknownKID
	}
	return mkid, ver, k, nil
}

func (svc *Service) SealingKey(ctx context.Context, appID, mkid string, keyVersion int) ([]byte, error) {
	tn, err := svc.q.GetApp(ctx, appID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if int64(keyVersion) < tn.MinKeyVersion {
		return nil, ErrKeyRevoked
	}
	key, ok := svc.keyring.DeriveEndpointKey(mkid, appID, keyVersion)
	if !ok {
		return nil, ErrUnknownKID
	}
	return key, nil
}

// tokenAAD is the domain tag for the sealed app-token blob. The app id
// lives inside the ciphertext, not in the AAD, so a leaked token reveals nothing.
const tokenAAD = "app:token"

func newSecret() ([]byte, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// buildToken produces an opaque token: "pat_" + base64url(Seal(appID‖0x00‖secret)).
func (svc *Service) buildToken(appID string, secret []byte) (string, error) {
	pt := make([]byte, 0, len(appID)+1+len(secret))
	pt = append(pt, appID...)
	pt = append(pt, 0)
	pt = append(pt, secret...)
	blob, err := svc.keyring.Seal(pt, []byte(tokenAAD))
	if err != nil {
		return "", err
	}
	return "pat_" + base64.RawURLEncoding.EncodeToString(blob), nil
}

// parseToken reverses buildToken; ok is false for any malformed or non-openable
// token (never a hard error — a bad token is simply unauthenticated).
func (svc *Service) parseToken(raw string) (appID string, secret []byte, ok bool) {
	enc, ok := strings.CutPrefix(raw, "pat_")
	if !ok {
		return "", nil, false
	}
	blob, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return "", nil, false
	}
	pt, err := svc.keyring.Open(blob, []byte(tokenAAD))
	if err != nil {
		return "", nil, false
	}
	i := bytes.IndexByte(pt, 0)
	if i < 0 {
		return "", nil, false
	}
	return string(pt[:i]), pt[i+1:], true
}

func verifierAAD(appID string) []byte { return []byte(appID + ":token") }

// sealVerifier stores sha256(secret) sealed under an id-bound AAD, so rotating the
// secret invalidates the old token while the id stays stable.
func (svc *Service) sealVerifier(appID string, secret []byte) ([]byte, error) {
	sum := sha256.Sum256(secret)
	return svc.keyring.Seal(sum[:], verifierAAD(appID))
}

// GenerateAppToken returns a new raw app token (shown once; only
// its sealed sha256 verifier is stored).
func (svc *Service) GenerateAppToken(ctx context.Context, appID string) (string, error) {
	secret, err := newSecret()
	if err != nil {
		return "", err
	}
	raw, err := svc.buildToken(appID, secret)
	if err != nil {
		return "", err
	}
	enc, err := svc.sealVerifier(appID, secret)
	if err != nil {
		return "", err
	}
	n, err := svc.q.SetAppToken(ctx, db.SetAppTokenParams{
		EncTokenHash: enc,
		ID:           appID,
	})
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", db.ErrNotFound
	}
	return raw, nil
}

// AuthenticateAppToken verifies a raw app token and resolves the app id;
// pathAppID may be "@app" to use the id embedded in the token. Invalid tokens and
// app-id mismatches return ok=false with nil error; non-nil error means storage
// failure.
func (svc *Service) AuthenticateAppToken(ctx context.Context, pathAppID, raw string) (string, bool, error) {
	tokenAppID, secret, ok := svc.parseToken(raw)
	if !ok {
		return "", false, nil
	}
	appID := pathAppID
	if pathAppID == "@app" {
		appID = tokenAppID
	} else if tokenAppID != pathAppID {
		return "", false, nil
	}
	tn, err := svc.q.GetApp(ctx, appID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if len(tn.EncTokenHash) == 0 {
		return "", false, nil
	}
	expected, err := svc.keyring.Open(tn.EncTokenHash, verifierAAD(appID))
	if err != nil {
		return "", false, nil
	}
	sum := sha256.Sum256(secret)
	if subtle.ConstantTimeCompare(expected, sum[:]) != 1 {
		return "", false, nil
	}
	return appID, true, nil
}

func (svc *Service) RotateEndpointKey(ctx context.Context, appID string, revokeOld bool) error {
	// Read-modify-write: InTx (BEGIN IMMEDIATE) holds the writer lock across the
	// read and write, so concurrent rotations serialize instead of clobbering each other.
	return svc.db.InTx(ctx, func(q *db.Queries) error {
		tn, err := q.GetApp(ctx, appID)
		if errors.Is(err, sql.ErrNoRows) {
			return db.ErrNotFound
		}
		if err != nil {
			return err
		}
		next := tn.KeyVersion + 1
		minv := tn.MinKeyVersion
		if revokeOld {
			minv = next
		}
		n, err := q.UpdateAppKeyVersion(ctx, db.UpdateAppKeyVersionParams{ID: appID, KeyVersion: next, MinKeyVersion: minv})
		return notFound(n, err)
	})
}
