package app

import (
	"context"
	"database/sql"
	"errors"

	"github.com/MunifTanjim/pushport/internal/db"
)

func turnstileAAD(appID string) []byte { return []byte(appID + ":turnstile") }

func (svc *Service) SetTurnstile(ctx context.Context, appID, siteKey, secret string) error {
	enc, err := svc.keyring.Seal([]byte(secret), turnstileAAD(appID))
	if err != nil {
		return err
	}
	return notFound(svc.q.SetAppTurnstile(ctx, db.SetAppTurnstileParams{
		TurnstileSiteKey:   db.NewNullString(siteKey),
		EncTurnstileSecret: enc,
		ID:                 appID,
	}))
}

func (svc *Service) DeleteTurnstile(ctx context.Context, appID string) error {
	return notFound(svc.q.ClearAppTurnstile(ctx, appID))
}

// GetTurnstile returns the app's Turnstile site key + secret. ok is false when
// the app exists but has no Turnstile configured; db.ErrNotFound if no such app.
func (svc *Service) GetTurnstile(ctx context.Context, appID string) (siteKey, secret string, ok bool, err error) {
	tn, err := svc.q.GetApp(ctx, appID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, db.ErrNotFound
	}
	if err != nil {
		return "", "", false, err
	}
	if !tn.TurnstileSiteKey.Valid || tn.TurnstileSiteKey.String == "" || len(tn.EncTurnstileSecret) == 0 {
		return "", "", false, nil
	}
	pt, err := svc.keyring.Open(tn.EncTurnstileSecret, turnstileAAD(appID))
	if err != nil {
		return "", "", false, err
	}
	return tn.TurnstileSiteKey.String, string(pt), true, nil
}
