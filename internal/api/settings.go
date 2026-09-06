package api

import (
	"net/http"
	"time"

	"github.com/MunifTanjim/pushport/internal/db"
)

type serverSettingView struct {
	RetryMaxAttempts int    `json:"retry_max_attempts"`
	RetryBaseBackoff string `json:"retry_base_backoff"`
	RetryMaxBackoff  string `json:"retry_max_backoff"`
	AuthFailIPPerMin int    `json:"auth_fail_ip_per_min"`
	AuthFailIPBurst  int    `json:"auth_fail_ip_burst"`
}

func toServerSettingView(s db.ServerSetting) serverSettingView {
	return serverSettingView{
		RetryMaxAttempts: int(s.RetryMaxAttempts),
		RetryBaseBackoff: s.RetryBaseBackoff,
		RetryMaxBackoff:  s.RetryMaxBackoff,
		AuthFailIPPerMin: int(s.AuthFailIpPerMin),
		AuthFailIPBurst:  int(s.AuthFailIpBurst),
	}
}

func (h *ManagementHandler) settingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /settings", requireAdmin(h.adminToken, h.getServerSetting))
	mux.HandleFunc("PATCH /settings", requireAdmin(h.adminToken, h.updateServerSetting))
}

func (h *ManagementHandler) getServerSetting(w http.ResponseWriter, r *http.Request) {
	s, err := h.queries.GetServerSetting(r.Context())
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	SendData(w, r, http.StatusOK, toServerSettingView(s))
}

func (h *ManagementHandler) updateServerSetting(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RetryMaxAttempts *int    `json:"retry_max_attempts"`
		RetryBaseBackoff *string `json:"retry_base_backoff"`
		RetryMaxBackoff  *string `json:"retry_max_backoff"`
		AuthFailIPPerMin *int    `json:"auth_fail_ip_per_min"`
		AuthFailIPBurst  *int    `json:"auth_fail_ip_burst"`
	}
	if err := ReadRequestBodyJSON(w, r, &body); err != nil {
		SendError(w, r, err)
		return
	}
	// Backoffs are stored as Go duration strings.
	for _, d := range []*string{body.RetryBaseBackoff, body.RetryMaxBackoff} {
		if d == nil {
			continue
		}
		dur, err := time.ParseDuration(*d)
		if err != nil {
			SendError(w, r, ErrorBadRequest().WithMessage("invalid backoff duration: "+*d))
			return
		}
		if dur <= 0 {
			SendError(w, r, ErrorBadRequest().WithMessage("backoff duration must be positive: "+*d))
			return
		}
	}
	if body.RetryMaxAttempts != nil && *body.RetryMaxAttempts < 1 {
		SendError(w, r, ErrorBadRequest().WithMessage("retry_max_attempts must be >= 1"))
		return
	}
	if e := requireNonNegative(
		namedLimit{"auth_fail_ip_per_min", body.AuthFailIPPerMin},
		namedLimit{"auth_fail_ip_burst", body.AuthFailIPBurst},
	); e != nil {
		SendError(w, r, e)
		return
	}
	if err := h.queries.UpdateServerSetting(r.Context(), db.UpdateServerSettingParams{
		RetryMaxAttempts: db.NewNullInt64(body.RetryMaxAttempts),
		RetryBaseBackoff: db.NewNullString(body.RetryBaseBackoff),
		RetryMaxBackoff:  db.NewNullString(body.RetryMaxBackoff),
		AuthFailIpPerMin: db.NewNullInt64(body.AuthFailIPPerMin),
		AuthFailIpBurst:  db.NewNullInt64(body.AuthFailIPBurst),
	}); err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if h.onSettingsChange != nil {
		h.onSettingsChange()
	}
	SendData(w, r, http.StatusNoContent, nil)
}
