package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/MunifTanjim/pushport/internal/db"
)

func (h *ManagementHandler) instanceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /apps/{app_id}/instances", h.ta(h.listInstances))
	mux.HandleFunc("GET /apps/{app_id}/instances/{instance_id}", h.ta(h.getInstance))
	mux.HandleFunc("PATCH /apps/{app_id}/instances/{instance_id}", h.ta(h.updateInstance))
	mux.HandleFunc("DELETE /apps/{app_id}/instances/{instance_id}", h.ta(h.deleteInstance))
	mux.HandleFunc("POST /apps/{app_id}/instances/{instance_id}/rotate-token", h.ta(h.rotateInstanceToken))
}

type instanceView struct {
	ID          string  `json:"id"`
	AppID       string  `json:"app_id"`
	Label       string  `json:"label"`
	UsagePlanID *string `json:"usage_plan_id"`
	CreatedAt   int64   `json:"created_at"`
}

func toInstanceView(k db.Instance) instanceView {
	return instanceView{
		ID:          k.ID,
		AppID:       k.AppID,
		Label:       k.Label,
		UsagePlanID: k.UsagePlanID.ToStrPtr(),
		CreatedAt:   k.CreatedAt,
	}
}

func (h *ManagementHandler) listInstances(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	rows, err := h.queries.ListInstancesByApp(r.Context(), appID)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	out := make([]instanceView, 0, len(rows))
	for _, k := range rows {
		out = append(out, toInstanceView(k))
	}
	SendData(w, r, http.StatusOK, out)
}

func (h *ManagementHandler) getInstance(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	iid := r.PathValue("instance_id")
	k, err := h.queries.GetInstance(r.Context(), iid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound())
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if k.AppID != appID {
		SendError(w, r, ErrorForbidden())
		return
	}
	SendData(w, r, http.StatusOK, toInstanceView(k))
}

// updateInstance applies partial label and usage-plan updates. usage_plan_id: a
// value assigns, "" clears, absent is a no-op.
func (h *ManagementHandler) updateInstance(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	iid := r.PathValue("instance_id")
	var body struct {
		Label       *string `json:"label"`
		UsagePlanID *string `json:"usage_plan_id"`
	}
	if err := ReadRequestBodyJSON(w, r, &body); err != nil {
		SendError(w, r, err)
		return
	}
	if body.Label != nil && *body.Label == "" {
		SendError(w, r, ErrorBadRequest().WithMessage("label must not be empty"))
		return
	}
	if body.Label == nil && body.UsagePlanID == nil {
		SendError(w, r, ErrorBadRequest().WithMessage("no fields to update"))
		return
	}
	k, err := h.queries.GetInstance(r.Context(), iid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound())
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if k.AppID != appID {
		SendError(w, r, ErrorForbidden())
		return
	}
	if body.UsagePlanID != nil {
		assign := db.NullString{}
		if *body.UsagePlanID != "" {
			p, err := h.queries.GetInstanceUsagePlan(r.Context(), *body.UsagePlanID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					SendError(w, r, ErrorNotFound().WithMessage("plan not found"))
					return
				}
				SendError(w, r, ErrorInternalServerError().WithCause(err))
				return
			}
			if p.AppID != appID || (p.InstanceID.Valid && p.InstanceID.String != iid) {
				SendError(w, r, ErrorNotFound().WithMessage("plan not found for app"))
				return
			}
			assign = db.NewNullString(body.UsagePlanID)
		}
		if _, err := h.queries.AssignInstanceUsagePlan(r.Context(), db.AssignInstanceUsagePlanParams{
			ID: iid, UsagePlanID: assign,
		}); err != nil {
			SendError(w, r, ErrorInternalServerError().WithCause(err))
			return
		}
		if h.onRateConfigChange != nil {
			h.onRateConfigChange(appID)
		}
	}
	if body.Label != nil {
		params := db.UpdateInstanceParams{
			ID: iid,
		}
		params.Label = db.NewNullString(body.Label)
		n, err := h.queries.UpdateInstance(r.Context(), params)
		if err != nil {
			SendError(w, r, ErrorInternalServerError().WithCause(err))
			return
		}
		if n == 0 {
			SendError(w, r, ErrorNotFound())
			return
		}
	}
	SendData(w, r, http.StatusNoContent, nil)
}

func (h *ManagementHandler) deleteInstance(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	iid := r.PathValue("instance_id")
	k, err := h.queries.GetInstance(r.Context(), iid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound())
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if k.AppID != appID {
		SendError(w, r, ErrorForbidden())
		return
	}
	if err := h.instances.Delete(r.Context(), iid); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			SendError(w, r, ErrorNotFound())
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	SendData(w, r, http.StatusNoContent, nil)
}

func (h *ManagementHandler) rotateInstanceToken(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	iid := r.PathValue("instance_id")
	k, err := h.queries.GetInstance(r.Context(), iid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound())
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if k.AppID != appID {
		SendError(w, r, ErrorForbidden())
		return
	}
	raw, err := h.instances.RotateToken(r.Context(), iid)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			SendError(w, r, ErrorNotFound())
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	SendData(w, r, http.StatusOK, map[string]string{"token": raw})
}
