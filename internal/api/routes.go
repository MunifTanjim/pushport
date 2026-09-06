package api

import (
	"net/http"

	"github.com/MunifTanjim/pushport/internal/app"
	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/instance"
)

type ManagementHandler struct {
	apps               *app.Service
	instances          *instance.Service
	queries            *db.Queries
	adminToken         string
	onCredsChange      func(appID string) // optional; set by wiring to bust the dispatcher cache
	onRateConfigChange func(appID string) // optional; set by wiring to bust the rate-limit resolver cache
	onSettingsChange   func()             // optional; set by wiring to bust the settings reader cache
}

func NewManagementHandler(a *app.Service, k *instance.Service, queries *db.Queries, adminToken string) *ManagementHandler {
	return &ManagementHandler{apps: a, instances: k, queries: queries, adminToken: adminToken}
}

func (h *ManagementHandler) OnCredsChange(fn func(string)) {
	h.onCredsChange = fn
}

func (h *ManagementHandler) OnRateConfigChange(fn func(string)) {
	h.onRateConfigChange = fn
}

func (h *ManagementHandler) OnSettingsChange(fn func()) {
	h.onSettingsChange = fn
}

// Routes registers all admin/management routes via per-topic registrars.
func (h *ManagementHandler) Routes(mux *http.ServeMux) {
	h.appRoutes(mux)
	h.instanceRoutes(mux)
	h.credRoutes(mux)
	h.turnstileRoutes(mux)
	h.tokenRoutes(mux)
	h.usagePlanRoutes(mux)
	h.settingsRoutes(mux)
	h.statsRoutes(mux)
}

// ta wraps a handler for app-scoped management: reachable by the admin
// token OR the target app's own token.
func (h *ManagementHandler) ta(next http.HandlerFunc) http.HandlerFunc {
	return requireAppAuth(h.adminToken, h.apps, next)
}
