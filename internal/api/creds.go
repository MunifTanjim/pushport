package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sort"

	"github.com/MunifTanjim/pushport/internal/app"
	"github.com/MunifTanjim/pushport/internal/db"
)

func (h *ManagementHandler) credRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /apps/{app_id}/creds", h.ta(h.listCreds))
	mux.HandleFunc("PUT /apps/{app_id}/creds/apns", h.ta(setCredsHandler(h, h.apps.SetAPNsCredentials)))
	mux.HandleFunc("DELETE /apps/{app_id}/creds/apns", h.ta(h.deleteCreds("apns")))
	mux.HandleFunc("PUT /apps/{app_id}/creds/fcm", h.ta(setCredsHandler(h, h.apps.SetFCMCredentials)))
	mux.HandleFunc("DELETE /apps/{app_id}/creds/fcm", h.ta(h.deleteCreds("fcm")))
	mux.HandleFunc("PUT /apps/{app_id}/creds/webpush", h.ta(setCredsHandler(h, h.apps.SetWebPushCredentials)))
	mux.HandleFunc("DELETE /apps/{app_id}/creds/webpush", h.ta(h.deleteCreds("webpush")))
}

type credsView struct {
	Transports []string `json:"transports"`
}

func (h *ManagementHandler) listCreds(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	// 404 unknown apps to match the sibling GET endpoints.
	if _, err := h.queries.GetApp(r.Context(), appID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound().WithMessage("app not found"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	rows, err := h.queries.ListAppCredentials(r.Context(), appID)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	transports := make([]string, 0, len(rows))
	for _, row := range rows {
		transports = append(transports, row.Transport)
	}
	sort.Strings(transports)
	SendData(w, r, http.StatusOK, credsView{Transports: transports})
}

func setCredsHandler[T any](h *ManagementHandler, set func(context.Context, string, T) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appID := r.PathValue("app_id")
		var creds T
		if err := ReadRequestBodyJSON(w, r, &creds, 64<<10); err != nil { // service-account JSON can be a few KB
			SendError(w, r, err)
			return
		}
		if err := set(r.Context(), appID, creds); err != nil {
			switch {
			case errors.Is(err, db.ErrNotFound):
				SendError(w, r, ErrorNotFound().WithMessage("app not found"))
			case errors.Is(err, app.ErrInvalidCreds):
				SendError(w, r, ErrorBadRequest().WithCause(err))
			default:
				SendError(w, r, ErrorInternalServerError().WithCause(err))
			}
			return
		}
		if h.onCredsChange != nil {
			h.onCredsChange(appID)
		}
		SendData(w, r, http.StatusNoContent, nil)
	}
}

func (h *ManagementHandler) deleteCreds(transport string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appID := r.PathValue("app_id")
		if err := h.apps.DeleteCredentials(r.Context(), appID, transport); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				SendError(w, r, ErrorNotFound().WithMessage("app not found"))
				return
			}
			SendError(w, r, ErrorInternalServerError().WithCause(err))
			return
		}
		if h.onCredsChange != nil {
			h.onCredsChange(appID)
		}
		SendData(w, r, http.StatusNoContent, nil)
	}
}
