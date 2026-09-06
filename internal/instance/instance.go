package instance

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

var ErrInvalid = errors.New("instance: invalid token")

type Service struct {
	q       *db.Queries
	keyring *crypto.Keyring
}

func NewService(q *db.Queries, kr *crypto.Keyring) *Service {
	return &Service{q: q, keyring: kr}
}

// tokenAAD is the domain tag for sealed instance tokens, so one can never be
// opened as some other kind of sealed blob. The id lives inside the ciphertext,
// not the AAD.
const tokenAAD = "instance:token"

func newSecret() ([]byte, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// buildToken produces an opaque token: prefix + base64url(Seal(id‖0x00‖secret)) —
// a leaked token reveals nothing about the instance it belongs to.
func (svc *Service) buildToken(instID string, secret []byte) (string, error) {
	pt := make([]byte, 0, len(instID)+1+len(secret))
	pt = append(pt, instID...)
	pt = append(pt, 0)
	pt = append(pt, secret...)
	blob, err := svc.keyring.Seal(pt, []byte(tokenAAD))
	if err != nil {
		return "", err
	}
	return "pit_" + base64.RawURLEncoding.EncodeToString(blob), nil
}

// parseToken reverses buildToken; ok is false for any malformed or non-openable
// token (never a hard error — a bad token is simply unauthenticated).
func (svc *Service) parseToken(raw string) (instID string, secret []byte, ok bool) {
	enc, ok := strings.CutPrefix(raw, "pit_")
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

func verifierAAD(instID string) []byte { return []byte("instance:" + instID + ":token") }

// sealVerifier stores sha256(secret) sealed under an id-bound AAD, so rotating
// the secret invalidates the old token while the id stays stable.
func (svc *Service) sealVerifier(instID string, secret []byte) ([]byte, error) {
	sum := sha256.Sum256(secret)
	return svc.keyring.Seal(sum[:], verifierAAD(instID))
}

func (svc *Service) Issue(ctx context.Context, appID, label string) (string, db.Instance, error) {
	instID := id.New()
	secret, err := newSecret()
	if err != nil {
		return "", db.Instance{}, err
	}
	raw, err := svc.buildToken(instID, secret)
	if err != nil {
		return "", db.Instance{}, err
	}
	enc, err := svc.sealVerifier(instID, secret)
	if err != nil {
		return "", db.Instance{}, err
	}
	now := time.Now().Unix()
	rec := db.Instance{
		ID:           instID,
		AppID:        appID,
		Label:        label,
		EncTokenHash: enc,
		CreatedAt:    now,
	}
	if err := svc.q.CreateInstance(ctx, db.CreateInstanceParams{
		ID:           rec.ID,
		AppID:        rec.AppID,
		Label:        rec.Label,
		EncTokenHash: rec.EncTokenHash,
		CreatedAt:    rec.CreatedAt,
	}); err != nil {
		return "", db.Instance{}, err
	}
	return raw, rec, nil
}

func (svc *Service) Authenticate(ctx context.Context, raw string) (db.Instance, error) {
	instID, secret, ok := svc.parseToken(raw)
	if !ok {
		return db.Instance{}, ErrInvalid
	}
	rec, err := svc.q.GetInstance(ctx, instID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Instance{}, ErrInvalid
	}
	if err != nil {
		return db.Instance{}, err
	}
	expected, err := svc.keyring.Open(rec.EncTokenHash, verifierAAD(instID))
	if err != nil {
		return db.Instance{}, ErrInvalid
	}
	sum := sha256.Sum256(secret)
	if subtle.ConstantTimeCompare(expected, sum[:]) != 1 {
		return db.Instance{}, ErrInvalid
	}
	return rec, nil
}

// RotateToken replaces an instance's token; the old one stops authenticating
// immediately. Returns the raw token (shown once).
func (svc *Service) RotateToken(ctx context.Context, id string) (string, error) {
	secret, err := newSecret()
	if err != nil {
		return "", err
	}
	raw, err := svc.buildToken(id, secret)
	if err != nil {
		return "", err
	}
	enc, err := svc.sealVerifier(id, secret)
	if err != nil {
		return "", err
	}
	n, err := svc.q.UpdateInstanceTokenHash(ctx, db.UpdateInstanceTokenHashParams{
		EncTokenHash: enc,
		ID:           id,
	})
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", db.ErrNotFound
	}
	return raw, nil
}

func (svc *Service) Delete(ctx context.Context, id string) error {
	n, err := svc.q.DeleteInstance(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return db.ErrNotFound
	}
	return nil
}
