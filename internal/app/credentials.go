package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/MunifTanjim/pushport/internal/db"
)

// ErrInvalidCreds marks caller-supplied credential payloads that are
// structurally unusable (bad JSON, missing required fields).
var ErrInvalidCreds = errors.New("app: invalid credentials")

type Credentials struct {
	APNs    *APNsCreds    `json:"apns,omitempty"`
	FCM     *FCMCreds     `json:"fcm,omitempty"`
	WebPush *WebPushCreds `json:"webpush,omitempty"`
}

type APNsCreds struct {
	KeyP8  string `json:"key_p8"`
	KeyID  string `json:"key_id"`
	TeamID string `json:"team_id"`
	Topic  string `json:"topic"`
}

type FCMCreds struct {
	ServiceAccountJSON string `json:"service_account_json"`
}

// FCMProjectID extracts the project_id from a service-account JSON. FCM v1
// send URLs are project-scoped, and the service account is issued per-project,
// so the id always lives inside the JSON itself.
func FCMProjectID(serviceAccountJSON string) (string, error) {
	var sa struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal([]byte(serviceAccountJSON), &sa); err != nil {
		return "", fmt.Errorf("%w: invalid service-account JSON: %v", ErrInvalidCreds, err)
	}
	if sa.ProjectID == "" {
		return "", fmt.Errorf("%w: service-account JSON has no project_id", ErrInvalidCreds)
	}
	return sa.ProjectID, nil
}

type WebPushCreds struct {
	VAPIDPrivateKey string `json:"vapid_private_key"`
	Subject         string `json:"subject"`
}

// credAAD is the per-transport AAD, so sealed blobs can't be swapped between
// transports or apps.
func credAAD(appID, transport string) []byte { return []byte(appID + ":creds:" + transport) }

// notFound maps an :execrows result to db.ErrNotFound when no row matched.
func notFound(n int64, err error) error {
	if err != nil {
		return err
	}
	if n == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (svc *Service) assertApp(ctx context.Context, appID string) error {
	if _, err := svc.q.GetApp(ctx, appID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.ErrNotFound
		}
		return err
	}
	return nil
}

func (svc *Service) setCred(ctx context.Context, appID, transport string, v any) error {
	if err := svc.assertApp(ctx, appID); err != nil {
		return err
	}
	pt, err := json.Marshal(v)
	if err != nil {
		return err
	}
	enc, err := svc.keyring.Seal(pt, credAAD(appID, transport))
	if err != nil {
		return err
	}
	return svc.q.UpsertAppCredential(ctx, db.UpsertAppCredentialParams{AppID: appID, Transport: transport, EncBlob: enc})
}

func (svc *Service) SetAPNsCredentials(ctx context.Context, appID string, c APNsCreds) error {
	return svc.setCred(ctx, appID, "apns", c)
}

func (svc *Service) SetFCMCredentials(ctx context.Context, appID string, c FCMCreds) error {
	if _, err := FCMProjectID(c.ServiceAccountJSON); err != nil {
		return err
	}
	return svc.setCred(ctx, appID, "fcm", c)
}

func (svc *Service) SetWebPushCredentials(ctx context.Context, appID string, c WebPushCreds) error {
	return svc.setCred(ctx, appID, "webpush", c)
}

// DeleteCredentials removes one transport's credential row (idempotent).
func (svc *Service) DeleteCredentials(ctx context.Context, appID, transport string) error {
	switch transport {
	case "apns", "fcm", "webpush":
	default:
		return errors.New("app: unknown transport " + transport)
	}
	if err := svc.assertApp(ctx, appID); err != nil {
		return err
	}
	return svc.q.DeleteAppCredential(ctx, db.DeleteAppCredentialParams{AppID: appID, Transport: transport})
}

// GetCredentials returns the app's per-transport credentials. An app with none
// configured (or an unknown app) yields an empty Credentials and no error.
func (svc *Service) GetCredentials(ctx context.Context, appID string) (Credentials, error) {
	rows, err := svc.q.ListAppCredentials(ctx, appID)
	if err != nil {
		return Credentials{}, err
	}
	var c Credentials
	for _, row := range rows {
		switch row.Transport {
		case "apns":
			var a APNsCreds
			if err := svc.openCred(appID, "apns", row.EncBlob, &a); err != nil {
				return Credentials{}, err
			}
			c.APNs = &a
		case "fcm":
			var f FCMCreds
			if err := svc.openCred(appID, "fcm", row.EncBlob, &f); err != nil {
				return Credentials{}, err
			}
			c.FCM = &f
		case "webpush":
			var w WebPushCreds
			if err := svc.openCred(appID, "webpush", row.EncBlob, &w); err != nil {
				return Credentials{}, err
			}
			c.WebPush = &w
		}
	}
	return c, nil
}

func (svc *Service) openCred(appID, transport string, enc []byte, v any) error {
	pt, err := svc.keyring.Open(enc, credAAD(appID, transport))
	if err != nil {
		return err
	}
	return json.Unmarshal(pt, v)
}
