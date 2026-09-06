package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/MunifTanjim/pushport/internal/db"
)

func (h *ManagementHandler) appRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /apps", requireAdmin(h.adminToken, h.listApps))
	mux.HandleFunc("POST /apps", requireAdmin(h.adminToken, h.createApp))
	mux.HandleFunc("GET /apps/{app_id}", h.ta(h.getApp))
	mux.HandleFunc("PATCH /apps/{app_id}", h.ta(h.updateApp))
}

type appView struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	IsPublic         bool    `json:"is_public"`
	KeyVersion       int64   `json:"key_version"`
	MinKeyVersion    int64   `json:"min_key_version"`
	UsagePlanID      *string `json:"usage_plan_id"`
	TurnstileSiteKey *string `json:"turnstile_site_key"`
	CreatedAt        int64   `json:"created_at"`
}

func toAppView(a db.App) appView {
	return appView{
		ID:               a.ID,
		Name:             a.Name,
		IsPublic:         a.IsPublic,
		KeyVersion:       a.KeyVersion,
		MinKeyVersion:    a.MinKeyVersion,
		UsagePlanID:      a.UsagePlanID.ToStrPtr(),
		TurnstileSiteKey: a.TurnstileSiteKey.ToStrPtr(),
		CreatedAt:        a.CreatedAt,
	}
}

func (h *ManagementHandler) listApps(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListApps(r.Context())
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	out := make([]appView, 0, len(rows))
	for _, a := range rows {
		out = append(out, toAppView(a))
	}
	SendData(w, r, http.StatusOK, out)
}

func (h *ManagementHandler) getApp(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	a, err := h.queries.GetApp(r.Context(), appID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound())
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	SendData(w, r, http.StatusOK, toAppView(a))
}

func (h *ManagementHandler) createApp(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		IsPublic bool   `json:"is_public"`
	}
	if err := ReadRequestBodyJSON(w, r, &body); err != nil {
		SendError(w, r, err)
		return
	}
	if body.Name == "" {
		SendError(w, r, ErrorBadRequest().WithMessage("name is required"))
		return
	}
	ctx := r.Context()
	a, err := h.apps.Create(ctx, body.Name)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if body.IsPublic {
		if err := h.apps.SetPublic(ctx, a.ID, true); err != nil {
			SendError(w, r, ErrorInternalServerError().WithCause(err))
			return
		}
	}
	raw, err := h.apps.GenerateAppToken(ctx, a.ID)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	SendData(w, r, http.StatusCreated, map[string]string{
		"id":    a.ID,
		"name":  a.Name,
		"token": raw,
	})
}

func (h *ManagementHandler) updateApp(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	var body struct {
		Name        *string `json:"name"`
		IsPublic    *bool   `json:"is_public"`
		UsagePlanID *string `json:"usage_plan_id"`
	}
	if err := ReadRequestBodyJSON(w, r, &body); err != nil {
		SendError(w, r, err)
		return
	}
	if body.Name != nil && *body.Name == "" {
		SendError(w, r, ErrorBadRequest().WithMessage("name must not be empty"))
		return
	}
	if body.Name == nil && body.IsPublic == nil && body.UsagePlanID == nil {
		SendError(w, r, ErrorBadRequest().WithMessage("no fields to update"))
		return
	}
	// Plan assignment is admin-only: an app must not set its own usage plan.
	if body.UsagePlanID != nil && !CallerIsAdmin(r) {
		SendError(w, r, ErrorForbidden().WithMessage("usage_plan_id requires the admin token"))
		return
	}
	// Validate the plan before any write, so a bad plan id can't leave a
	// half-applied update behind.
	assign := db.NullString{}
	if body.UsagePlanID != nil && *body.UsagePlanID != "" {
		p, err := h.queries.GetAppUsagePlan(r.Context(), *body.UsagePlanID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				SendError(w, r, ErrorNotFound().WithMessage("plan not found"))
				return
			}
			SendError(w, r, ErrorInternalServerError().WithCause(err))
			return
		}
		if p.AppID.Valid && p.AppID.String != appID {
			SendError(w, r, ErrorForbidden().WithMessage("plan belongs to another app"))
			return
		}
		assign = db.NewNullString(body.UsagePlanID)
	}
	if body.Name != nil || body.IsPublic != nil {
		if err := h.apps.Update(r.Context(), appID, body.Name, body.IsPublic); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				SendError(w, r, ErrorNotFound().WithMessage("app not found"))
				return
			}
			SendError(w, r, ErrorInternalServerError().WithCause(err))
			return
		}
	}
	if body.UsagePlanID != nil {
		n, err := h.queries.AssignAppUsagePlan(r.Context(), db.AssignAppUsagePlanParams{
			ID:          appID,
			UsagePlanID: assign,
		})
		if err != nil {
			SendError(w, r, ErrorInternalServerError().WithCause(err))
			return
		}
		if n == 0 {
			SendError(w, r, ErrorNotFound().WithMessage("app not found"))
			return
		}
		if h.onRateConfigChange != nil {
			h.onRateConfigChange(appID)
		}
	}
	SendData(w, r, http.StatusNoContent, nil)
}
