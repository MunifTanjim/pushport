package api

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strings"

	"github.com/MunifTanjim/pushport/internal/app"
	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/instance"
	"github.com/MunifTanjim/pushport/internal/ratelimit"
	"github.com/MunifTanjim/pushport/internal/turnstile"
)

//go:embed register.html.tmpl
var registerPageFS embed.FS

var registerPageTmpl = template.Must(template.ParseFS(registerPageFS, "register.html.tmpl"))

type InstanceRegistrationHandler struct {
	instances  *instance.Service
	apps       *app.Service
	limiter    *ratelimit.Limiter
	resolver   *ratelimit.Resolver
	adminToken string
	verifier   turnstile.Verifier

	ipLimiter *ratelimit.Limiter
}

// SetIPRateLimit installs a per-client-IP floor on public self-registration so
// one attacker cannot drain an app's whole shared budget. A nil limiter disables it.
func (h *InstanceRegistrationHandler) SetIPRateLimit(l *ratelimit.Limiter) { h.ipLimiter = l }

func NewInstanceRegistrationHandler(instances *instance.Service, apps *app.Service, limiter *ratelimit.Limiter, resolver *ratelimit.Resolver, adminToken string, verifier turnstile.Verifier) *InstanceRegistrationHandler {
	return &InstanceRegistrationHandler{
		instances: instances, apps: apps, limiter: limiter, resolver: resolver,
		adminToken: adminToken, verifier: verifier,
	}
}

func (h *InstanceRegistrationHandler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /apps/{app_id}/instances/register", h.registerPage)
	mux.HandleFunc("POST /apps/{app_id}/instances", h.createInstance)
}

func (h *InstanceRegistrationHandler) resolveTurnstile(ctx context.Context, appID string) (siteKey, secret string, enabled bool, err error) {
	site, sec, ok, err := h.apps.GetTurnstile(ctx, appID)
	if err != nil {
		return "", "", false, err
	}
	if ok {
		return site, sec, true, nil
	}
	return "", "", false, nil
}

// Per-IP register-page ceiling, checked before any DB lookup so app-id scans
// can't drive unbounded queries.
const (
	registerPageIPPerMin = 60
	registerPageIPBurst  = 10
)

func (h *InstanceRegistrationHandler) registerPage(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	ctx := r.Context()
	if h.ipLimiter != nil {
		if ok, retry := h.ipLimiter.Allow("registerpage:"+clientIP(r), registerPageIPPerMin, registerPageIPBurst); !ok {
			e := ErrorTooManyRequests()
			if retry > 0 {
				e.WithRetryAfter(retryAfterSeconds(retry))
			}
			SendError(w, r, e)
			return
		}
	}
	pub, err := h.apps.IsPublic(ctx, appID)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	// Uniform response for "no such app" and "app exists but private" so the
	// page can't be probed to learn which private apps exist.
	if err != nil || !pub {
		SendError(w, r, ErrorNotFound().WithMessage("self-registration not available"))
		return
	}
	// Turnstile is optional: when unconfigured, siteKey is empty and the form
	// renders without a challenge widget. The POST path skips verification to
	// match, so registration stays gated by rate limits alone.
	siteKey, _, _, err := h.resolveTurnstile(ctx, appID)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = registerPageTmpl.Execute(w, map[string]string{
		"AppID":   appID,
		"SiteKey": siteKey,
		"PostURL": "/apps/" + appID + "/instances",
	})
}

func (h *InstanceRegistrationHandler) createInstance(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	ctx := r.Context()

	authed, present, resolved := authorizeApp(r, h.adminToken, h.apps)
	if present {
		if !authed {
			SendError(w, r, ErrorUnauthorized())
			return
		}
		appID = resolved
		if _, _, _, err := h.apps.CurrentKey(ctx, appID); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				SendError(w, r, ErrorNotFound().WithMessage("app not found"))
				return
			}
			SendError(w, r, ErrorInternalServerError().WithCause(err))
			return
		}
		label := decodeLabel(w, r)
		if strings.TrimSpace(label) == "" {
			SendError(w, r, ErrorBadRequest().WithMessage("label is required"))
			return
		}
		h.mint(w, r, appID, label)
		return
	}

	lim := h.resolver.Limits(ctx, appID, "")

	// Per-IP floor first: cheap single-source guard, before the Turnstile round-trip.
	if h.ipLimiter != nil {
		if ok, retry := h.ipLimiter.Allow(appID+":"+clientIP(r), lim.AppRegisterIPPerMin, lim.AppRegisterIPBurst); !ok {
			e := ErrorTooManyRequests()
			if retry > 0 {
				e.WithRetryAfter(retryAfterSeconds(retry))
			}
			SendError(w, r, e)
			return
		}
	}

	pub, err := h.apps.IsPublic(ctx, appID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			SendError(w, r, ErrorNotFound().WithMessage("app not found"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if !pub {
		SendError(w, r, ErrorForbidden().WithMessage("self-registration disabled"))
		return
	}
	_, secret, enabled, err := h.resolveTurnstile(ctx, appID)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	var body struct {
		Label          string `json:"label"`
		TurnstileToken string `json:"turnstile_token"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	_ = json.NewDecoder(r.Body).Decode(&body)
	if enabled {
		if body.TurnstileToken == "" {
			SendError(w, r, ErrorForbidden().WithMessage("turnstile required"))
			return
		}
		ok, verr := h.verifier.Verify(ctx, secret, body.TurnstileToken, clientIP(r))
		if verr != nil {
			SendError(w, r, ErrorBadGateway().WithMessage("turnstile verification unavailable").WithCause(verr))
			return
		}
		if !ok {
			SendError(w, r, ErrorForbidden().WithMessage("turnstile failed"))
			return
		}
	}

	if strings.TrimSpace(body.Label) == "" {
		SendError(w, r, ErrorBadRequest().WithMessage("label is required"))
		return
	}

	// App-global cap: consumed only after a solved challenge, so a flood of
	// unsolved requests cannot drain the shared bucket and DoS registration.
	if ok, retry := h.limiter.Allow(appID, lim.AppRegisterPerMin, lim.AppRegisterBurst); !ok {
		e := ErrorTooManyRequests()
		if retry > 0 {
			e.WithRetryAfter(retryAfterSeconds(retry))
		}
		SendError(w, r, e)
		return
	}

	h.mint(w, r, appID, body.Label+" ("+clientIP(r)+")")
}

// decodeLabel reads the optional label from the JSON body (best-effort).
func decodeLabel(w http.ResponseWriter, r *http.Request) string {
	var body struct {
		Label string `json:"label"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body.Label
}

func (h *InstanceRegistrationHandler) mint(w http.ResponseWriter, r *http.Request, appID, label string) {
	raw, rec, err := h.instances.Issue(r.Context(), appID, label)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	SendData(w, r, http.StatusCreated, map[string]string{"id": rec.ID, "token": raw})
}
