package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/MunifTanjim/pushport/internal/db"
)

func (h *ManagementHandler) turnstileRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /apps/{app_id}/turnstile", h.ta(h.getTurnstile))
	mux.HandleFunc("PUT /apps/{app_id}/turnstile", h.ta(h.setTurnstile))
	mux.HandleFunc("DELETE /apps/{app_id}/turnstile", h.ta(h.deleteTurnstile))
}

func (h *ManagementHandler) getTurnstile(w http.ResponseWriter, r *http.Request) {
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
	configured := a.TurnstileSiteKey.Valid && len(a.EncTurnstileSecret) > 0
	SendData(w, r, http.StatusOK, struct {
		Configured bool    `json:"configured"`
		SiteKey    *string `json:"site_key"`
	}{
		Configured: configured,
		SiteKey:    a.TurnstileSiteKey.ToStrPtr(),
	})
}

func (h *ManagementHandler) setTurnstile(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	var body struct {
		SiteKey   string `json:"site_key"`
		SecretKey string `json:"secret_key"`
	}
	if err := ReadRequestBodyJSON(w, r, &body); err != nil {
		SendError(w, r, err)
		return
	}
	if body.SiteKey == "" || body.SecretKey == "" {
		SendError(w, r, ErrorBadRequest().WithMessage("site_key and secret_key are required"))
		return
	}
	if err := h.apps.SetTurnstile(r.Context(), appID, body.SiteKey, body.SecretKey); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			SendError(w, r, ErrorNotFound().WithMessage("app not found"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	SendData(w, r, http.StatusNoContent, nil)
}

func (h *ManagementHandler) deleteTurnstile(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	if err := h.apps.DeleteTurnstile(r.Context(), appID); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			SendError(w, r, ErrorNotFound().WithMessage("app not found"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	SendData(w, r, http.StatusNoContent, nil)
}
