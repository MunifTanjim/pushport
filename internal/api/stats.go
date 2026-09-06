package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/ratelimit"
)

func (h *ManagementHandler) statsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /stats", requireAdmin(h.adminToken, h.getStats))
	mux.HandleFunc("GET /apps/{app_id}/stats", h.ta(h.getAppStats))
}

// statsView is the admin dashboard's snapshot for one UTC day. Push counts
// come from the durable usage_counter table, so they lag the in-memory quota
// store by up to one quota flush interval (PUSHPORT_QUOTA_FLUSH_INTERVAL, 30s
// by default).
type statsView struct {
	Date             string           `json:"date"`
	TotalApps        int64            `json:"total_apps"`
	TotalInstances   int64            `json:"total_instances"`
	TotalPushes      int64            `json:"total_pushes"`
	PushesByApp      map[string]int64 `json:"pushes_by_app"`
	PushesByInstance map[string]int64 `json:"pushes_by_instance"`
}

func (h *ManagementHandler) getStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	day := db.NewDate(time.Now())

	rows, err := h.queries.ListUsageCountersForDay(ctx, day)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	byApp := make(map[string]int64)
	byInstance := make(map[string]int64)
	var total int64
	for _, row := range rows {
		switch {
		case strings.HasPrefix(row.Scope, ratelimit.ScopeApp):
			byApp[strings.TrimPrefix(row.Scope, ratelimit.ScopeApp)] = row.Count
		case strings.HasPrefix(row.Scope, ratelimit.ScopeInstance):
			byInstance[strings.TrimPrefix(row.Scope, ratelimit.ScopeInstance)] = row.Count
		}
		total += row.Count
	}

	apps, err := h.queries.CountApps(ctx)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	instances, err := h.queries.CountInstances(ctx)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}

	SendData(w, r, http.StatusOK, statsView{
		Date:             day.String(),
		TotalApps:        apps,
		TotalInstances:   instances,
		TotalPushes:      total,
		PushesByApp:      byApp,
		PushesByInstance: byInstance,
	})
}

// appStatsView is one app's snapshot for the current UTC day. TotalPushes is the
// app-scope counter the gate increments on every push, so it lags the in-memory
// quota store by up to one flush interval, same as the admin dashboard.
type appStatsView struct {
	Date             string           `json:"date"`
	TotalInstances   int64            `json:"total_instances"`
	TotalPushes      int64            `json:"total_pushes"`
	PushesByInstance map[string]int64 `json:"pushes_by_instance"`
}

func (h *ManagementHandler) getAppStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	appID := r.PathValue("app_id")
	day := db.NewDate(time.Now())

	instances, err := h.queries.ListInstancesByApp(ctx, appID)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	ownInstance := make(map[string]struct{}, len(instances))
	for _, inst := range instances {
		ownInstance[inst.ID] = struct{}{}
	}

	rows, err := h.queries.ListUsageCountersForDay(ctx, day)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	var total int64
	byInstance := make(map[string]int64)
	for _, row := range rows {
		switch {
		case row.Scope == ratelimit.ScopeApp+appID:
			total = row.Count
		case strings.HasPrefix(row.Scope, ratelimit.ScopeInstance):
			id := strings.TrimPrefix(row.Scope, ratelimit.ScopeInstance)
			if _, ok := ownInstance[id]; ok {
				byInstance[id] = row.Count
			}
		}
	}

	SendData(w, r, http.StatusOK, appStatsView{
		Date:             day.String(),
		TotalInstances:   int64(len(instances)),
		TotalPushes:      total,
		PushesByInstance: byInstance,
	})
}
