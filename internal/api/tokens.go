package api

import (
	"errors"
	"net/http"

	"github.com/MunifTanjim/pushport/internal/db"
)

func (h *ManagementHandler) tokenRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /apps/{app_id}/rotate-token", h.ta(h.rotateAppToken))
	mux.HandleFunc("POST /apps/{app_id}/rotate-endpoint-key", h.ta(h.rotateEndpointKey))
}

func (h *ManagementHandler) rotateAppToken(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	raw, err := h.apps.GenerateAppToken(r.Context(), appID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			SendError(w, r, ErrorNotFound().WithMessage("app not found"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	SendData(w, r, http.StatusOK, map[string]string{"token": raw})
}

func (h *ManagementHandler) rotateEndpointKey(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	var body struct {
		RevokeOld bool `json:"revoke_old"`
	}
	if r.ContentLength != 0 {
		if err := ReadRequestBodyJSON(w, r, &body); err != nil {
			SendError(w, r, err)
			return
		}
	}
	if err := h.apps.RotateEndpointKey(r.Context(), appID, body.RevokeOld); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			SendError(w, r, ErrorNotFound().WithMessage("app not found"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	SendData(w, r, http.StatusNoContent, nil)
}
