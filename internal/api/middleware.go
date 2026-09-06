package api

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/MunifTanjim/pushport/internal/app"
)

func requireAdmin(token string, next http.HandlerFunc) http.HandlerFunc {
	expected := []byte("Bearer " + token)
	return func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, expected) != 1 {
			SendError(w, r, ErrorUnauthorized())
			return
		}
		next(w, r)
	}
}

// authorizeApp reports whether a bearer token is present and, if it is the
// admin token or the app's own token, authed with the
// resolved app id. "@app" resolves only from an app token, never the admin
// token (which is not bound to a single app).
func authorizeApp(r *http.Request, adminToken string, apps *app.Service) (authed, present bool, appID string) {
	raw, ok := bearer(r)
	if !ok {
		return false, false, ""
	}
	pathAppID := r.PathValue("app_id")
	if subtle.ConstantTimeCompare([]byte("Bearer "+raw), []byte("Bearer "+adminToken)) == 1 {
		if pathAppID == "@app" {
			return false, true, ""
		}
		return true, true, pathAppID
	}
	resolved, ok, err := apps.AuthenticateAppToken(r.Context(), pathAppID, raw)
	if err != nil || !ok {
		return false, true, ""
	}
	return true, true, resolved
}

// requireAppAuth allows the admin (any app) or the app's own token
// (that app only), rewriting the {app_id} path value to the resolved id so
// the "@app" placeholder is transparent downstream, and records CallerIsAdmin
// for field-level authorization.
func requireAppAuth(adminToken string, apps *app.Service, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authed, _, appID := authorizeApp(r, adminToken, apps)
		if !authed {
			SendError(w, r, ErrorUnauthorized())
			return
		}
		isAdmin := false
		if raw, ok := bearer(r); ok {
			isAdmin = subtle.ConstantTimeCompare([]byte("Bearer "+raw), []byte("Bearer "+adminToken)) == 1
		}
		r.SetPathValue("app_id", appID)
		next(w, r.WithContext(context.WithValue(r.Context(), callerIsAdminKey, isAdmin)))
	}
}
