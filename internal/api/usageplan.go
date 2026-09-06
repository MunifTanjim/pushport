package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/id"
)

// appUsagePlanView is the JSON representation of an app_usage_plan row. app_id
// is null for shared plans, set for plans scoped to a single app.
type appUsagePlanView struct {
	ID               string  `json:"id"`
	AppID            *string `json:"app_id"`
	Name             string  `json:"name"`
	IsDefault        bool    `json:"is_default"`
	PushPerMin       int     `json:"push_per_min"`
	PushDailyQuota   int     `json:"push_daily_quota"`
	RegisterPerMin   int     `json:"register_per_min"`
	RegisterBurst    int     `json:"register_burst"`
	RegisterIPPerMin int     `json:"register_ip_per_min"`
	RegisterIPBurst  int     `json:"register_ip_burst"`
	SubscribePerMin  int     `json:"subscribe_per_min"`
	SubscribeBurst   int     `json:"subscribe_burst"`
}

func toAppUsagePlanView(p db.AppUsagePlan) appUsagePlanView {
	return appUsagePlanView{
		ID:               p.ID,
		AppID:            p.AppID.ToStrPtr(),
		Name:             p.Name,
		IsDefault:        p.IsDefault,
		PushPerMin:       int(p.PushPerMin),
		PushDailyQuota:   int(p.PushDailyQuota),
		RegisterPerMin:   int(p.RegisterPerMin),
		RegisterBurst:    int(p.RegisterBurst),
		RegisterIPPerMin: int(p.RegisterIpPerMin),
		RegisterIPBurst:  int(p.RegisterIpBurst),
		SubscribePerMin:  int(p.SubscribePerMin),
		SubscribeBurst:   int(p.SubscribeBurst),
	}
}

func derefOr0(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

type namedLimit struct {
	name string
	v    *int
}

// requireNonNegative skips nil (unset) limits; 0 is allowed and means unlimited.
// Return type is *APIError (not error) so `if e := ...; e != nil` stays correct.
func requireNonNegative(limits ...namedLimit) *APIError {
	for _, l := range limits {
		if l.v != nil && *l.v < 0 {
			return ErrorBadRequest().WithMessage(l.name + " must be >= 0 (0 = unlimited)")
		}
	}
	return nil
}

func (h *ManagementHandler) usagePlanRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /usage-plans", requireAdmin(h.adminToken, h.createAppUsagePlan))
	mux.HandleFunc("GET /usage-plans", requireAdmin(h.adminToken, h.listAppUsagePlans))
	mux.HandleFunc("GET /usage-plans/{plan_id}", requireAdmin(h.adminToken, h.getAppUsagePlan))
	mux.HandleFunc("PATCH /usage-plans/{plan_id}", requireAdmin(h.adminToken, h.updateAppUsagePlan))
	mux.HandleFunc("DELETE /usage-plans/{plan_id}", requireAdmin(h.adminToken, h.deleteAppUsagePlan))
	mux.HandleFunc("GET /apps/{app_id}/usage-plan", h.ta(h.getAssignedAppUsagePlan))
	mux.HandleFunc("POST /apps/{app_id}/usage-plans", h.ta(h.createInstanceUsagePlan))
	mux.HandleFunc("GET /apps/{app_id}/usage-plans", h.ta(h.listInstanceUsagePlans))
	mux.HandleFunc("GET /apps/{app_id}/usage-plans/{plan_id}", h.ta(h.getInstanceUsagePlan))
	mux.HandleFunc("PATCH /apps/{app_id}/usage-plans/{plan_id}", h.ta(h.updateInstanceUsagePlan))
	mux.HandleFunc("DELETE /apps/{app_id}/usage-plans/{plan_id}", h.ta(h.deleteInstanceUsagePlan))
	mux.HandleFunc("GET /apps/{app_id}/instances/{instance_id}/usage-plan", h.ta(h.getAssignedInstanceUsagePlan))
}

func (h *ManagementHandler) getAppUsagePlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("plan_id")
	p, err := h.queries.GetAppUsagePlan(r.Context(), planID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound())
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	SendData(w, r, http.StatusOK, toAppUsagePlanView(p))
}

func (h *ManagementHandler) getInstanceUsagePlan(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	planID := r.PathValue("plan_id")
	p, err := h.queries.GetInstanceUsagePlan(r.Context(), planID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound())
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if p.AppID != appID {
		SendError(w, r, ErrorNotFound())
		return
	}
	SendData(w, r, http.StatusOK, toInstanceUsagePlanView(p))
}

func (h *ManagementHandler) createAppUsagePlan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AppID            *string `json:"app_id"`
		Name             string  `json:"name"`
		PushPerMin       *int    `json:"push_per_min"`
		PushDailyQuota   *int    `json:"push_daily_quota"`
		RegisterPerMin   *int    `json:"register_per_min"`
		RegisterBurst    *int    `json:"register_burst"`
		RegisterIPPerMin *int    `json:"register_ip_per_min"`
		RegisterIPBurst  *int    `json:"register_ip_burst"`
		SubscribePerMin  *int    `json:"subscribe_per_min"`
		SubscribeBurst   *int    `json:"subscribe_burst"`
	}
	if err := ReadRequestBodyJSON(w, r, &body); err != nil {
		SendError(w, r, err)
		return
	}
	if body.Name == "" {
		SendError(w, r, ErrorBadRequest().WithMessage("name is required"))
		return
	}
	if body.PushPerMin == nil || body.PushDailyQuota == nil {
		SendError(w, r, ErrorBadRequest().WithMessage("push_per_min and push_daily_quota are required"))
		return
	}
	if e := requireNonNegative(
		namedLimit{"push_per_min", body.PushPerMin},
		namedLimit{"push_daily_quota", body.PushDailyQuota},
		namedLimit{"register_per_min", body.RegisterPerMin},
		namedLimit{"register_burst", body.RegisterBurst},
		namedLimit{"register_ip_per_min", body.RegisterIPPerMin},
		namedLimit{"register_ip_burst", body.RegisterIPBurst},
		namedLimit{"subscribe_per_min", body.SubscribePerMin},
		namedLimit{"subscribe_burst", body.SubscribeBurst},
	); e != nil {
		SendError(w, r, e)
		return
	}
	// app_id scopes the plan to that app alone; omitted (or null) creates a
	// shared plan any app can be assigned to.
	appID := db.NullString{}
	if body.AppID != nil && *body.AppID != "" {
		if _, err := h.queries.GetApp(r.Context(), *body.AppID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				SendError(w, r, ErrorNotFound().WithMessage("app not found"))
				return
			}
			SendError(w, r, ErrorInternalServerError().WithCause(err))
			return
		}
		appID = db.NewNullString(body.AppID)
	}
	id := newPlanID()
	if err := h.queries.CreateAppUsagePlan(r.Context(), db.CreateAppUsagePlanParams{
		ID:               id,
		AppID:            appID,
		Name:             body.Name,
		PushPerMin:       int64(*body.PushPerMin),
		PushDailyQuota:   int64(*body.PushDailyQuota),
		RegisterPerMin:   int64(derefOr0(body.RegisterPerMin)),
		RegisterBurst:    int64(derefOr0(body.RegisterBurst)),
		RegisterIpPerMin: int64(derefOr0(body.RegisterIPPerMin)),
		RegisterIpBurst:  int64(derefOr0(body.RegisterIPBurst)),
		SubscribePerMin:  int64(derefOr0(body.SubscribePerMin)),
		SubscribeBurst:   int64(derefOr0(body.SubscribeBurst)),
		CreatedAt:        time.Now().Unix(),
	}); err != nil {
		if db.IsUniqueViolation(err) {
			SendError(w, r, ErrorConflict().WithMessage("app already has a scoped usage plan"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	p, err := h.queries.GetAppUsagePlan(r.Context(), id)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if h.onRateConfigChange != nil {
		h.onRateConfigChange(appID.String)
	}
	SendData(w, r, http.StatusCreated, toAppUsagePlanView(p))
}

func (h *ManagementHandler) listAppUsagePlans(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListAppUsagePlans(r.Context())
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	out := make([]appUsagePlanView, 0, len(rows))
	for _, p := range rows {
		out = append(out, toAppUsagePlanView(p))
	}
	SendData(w, r, http.StatusOK, out)
}

func (h *ManagementHandler) updateAppUsagePlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("plan_id")
	var body struct {
		Name             *string `json:"name"`
		PushPerMin       *int    `json:"push_per_min"`
		PushDailyQuota   *int    `json:"push_daily_quota"`
		RegisterPerMin   *int    `json:"register_per_min"`
		RegisterBurst    *int    `json:"register_burst"`
		RegisterIPPerMin *int    `json:"register_ip_per_min"`
		RegisterIPBurst  *int    `json:"register_ip_burst"`
		SubscribePerMin  *int    `json:"subscribe_per_min"`
		SubscribeBurst   *int    `json:"subscribe_burst"`
	}
	if err := ReadRequestBodyJSON(w, r, &body); err != nil {
		SendError(w, r, err)
		return
	}
	if e := requireNonNegative(
		namedLimit{"push_per_min", body.PushPerMin},
		namedLimit{"push_daily_quota", body.PushDailyQuota},
		namedLimit{"register_per_min", body.RegisterPerMin},
		namedLimit{"register_burst", body.RegisterBurst},
		namedLimit{"register_ip_per_min", body.RegisterIPPerMin},
		namedLimit{"register_ip_burst", body.RegisterIPBurst},
		namedLimit{"subscribe_per_min", body.SubscribePerMin},
		namedLimit{"subscribe_burst", body.SubscribeBurst},
	); e != nil {
		SendError(w, r, e)
		return
	}
	updateParams := db.UpdateAppUsagePlanParams{
		ID:               planID,
		PushPerMin:       db.NewNullInt64(body.PushPerMin),
		PushDailyQuota:   db.NewNullInt64(body.PushDailyQuota),
		RegisterPerMin:   db.NewNullInt64(body.RegisterPerMin),
		RegisterBurst:    db.NewNullInt64(body.RegisterBurst),
		RegisterIpPerMin: db.NewNullInt64(body.RegisterIPPerMin),
		RegisterIpBurst:  db.NewNullInt64(body.RegisterIPBurst),
		SubscribePerMin:  db.NewNullInt64(body.SubscribePerMin),
		SubscribeBurst:   db.NewNullInt64(body.SubscribeBurst),
	}
	updateParams.Name = db.NewNullString(body.Name)
	n, err := h.queries.UpdateAppUsagePlan(r.Context(), updateParams)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if n == 0 {
		SendError(w, r, ErrorNotFound())
		return
	}
	if h.onRateConfigChange != nil {
		h.onRateConfigChange("")
	}
	SendData(w, r, http.StatusNoContent, nil)
}

func (h *ManagementHandler) deleteAppUsagePlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("plan_id")
	// The global default app plan (synced from config) is undeletable.
	if p, err := h.queries.GetAppUsagePlan(r.Context(), planID); err == nil && p.IsDefault {
		SendError(w, r, ErrorForbidden().WithMessage("the default app plan cannot be deleted"))
		return
	}
	n, err := h.queries.DeleteAppUsagePlan(r.Context(), planID)
	if err != nil {
		if db.IsForeignKeyViolation(err) {
			SendError(w, r, ErrorConflict().WithMessage("plan is in use by one or more apps"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if n == 0 {
		SendError(w, r, ErrorNotFound().WithMessage("app usage plan not found"))
		return
	}
	if h.onRateConfigChange != nil {
		h.onRateConfigChange("")
	}
	SendData(w, r, http.StatusNoContent, nil)
}

// getAssignedAppUsagePlan returns the assigned plan, or the global default when
// none is assigned (the resolver's fallback order).
func (h *ManagementHandler) getAssignedAppUsagePlan(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	ctx := r.Context()
	app, err := h.queries.GetApp(ctx, appID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound().WithMessage("app not found"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	var p db.AppUsagePlan
	if app.UsagePlanID.Valid {
		p, err = h.queries.GetAppUsagePlan(ctx, app.UsagePlanID.String)
	} else {
		p, err = h.queries.GetDefaultAppUsagePlan(ctx)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound().WithMessage("no usage plan assigned"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	SendData(w, r, http.StatusOK, toAppUsagePlanView(p))
}

// instanceUsagePlanView is the JSON representation of an instance_usage_plan
// row. instance_id is null for plans shared across the app's instances, set for
// plans scoped to a single instance.
type instanceUsagePlanView struct {
	ID             string  `json:"id"`
	InstanceID     *string `json:"instance_id"`
	Name           string  `json:"name"`
	IsDefault      bool    `json:"is_default"`
	PushPerMin     int     `json:"push_per_min"`
	PushBurst      int     `json:"push_burst"`
	PushDailyQuota int     `json:"push_daily_quota"`
}

func toInstanceUsagePlanView(p db.InstanceUsagePlan) instanceUsagePlanView {
	return instanceUsagePlanView{
		ID:             p.ID,
		InstanceID:     p.InstanceID.ToStrPtr(),
		Name:           p.Name,
		IsDefault:      p.IsDefault,
		PushPerMin:     int(p.PushPerMin),
		PushBurst:      int(p.PushBurst),
		PushDailyQuota: int(p.PushDailyQuota),
	}
}

func (h *ManagementHandler) createInstanceUsagePlan(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	var body struct {
		InstanceID     *string `json:"instance_id"`
		Name           string  `json:"name"`
		PushPerMin     *int    `json:"push_per_min"`
		PushBurst      *int    `json:"push_burst"`
		PushDailyQuota *int    `json:"push_daily_quota"`
	}
	if err := ReadRequestBodyJSON(w, r, &body); err != nil {
		SendError(w, r, err)
		return
	}
	if body.Name == "" {
		SendError(w, r, ErrorBadRequest().WithMessage("name is required"))
		return
	}
	if body.PushPerMin == nil || body.PushBurst == nil || body.PushDailyQuota == nil {
		SendError(w, r, ErrorBadRequest().WithMessage("push_per_min, push_burst and push_daily_quota are required"))
		return
	}
	if e := requireNonNegative(
		namedLimit{"push_per_min", body.PushPerMin},
		namedLimit{"push_burst", body.PushBurst},
		namedLimit{"push_daily_quota", body.PushDailyQuota},
	); e != nil {
		SendError(w, r, e)
		return
	}
	// instance_id scopes the plan to that instance alone; omitted (or null)
	// creates a plan shared across the app's instances.
	instanceID := db.NullString{}
	if body.InstanceID != nil && *body.InstanceID != "" {
		inst, err := h.queries.GetInstance(r.Context(), *body.InstanceID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				SendError(w, r, ErrorNotFound().WithMessage("instance not found"))
				return
			}
			SendError(w, r, ErrorInternalServerError().WithCause(err))
			return
		}
		if inst.AppID != appID {
			SendError(w, r, ErrorNotFound())
			return
		}
		instanceID = db.NewNullString(body.InstanceID)
	}
	id := newPlanID()
	if err := h.queries.CreateInstanceUsagePlan(r.Context(), db.CreateInstanceUsagePlanParams{
		ID:             id,
		AppID:          appID,
		InstanceID:     instanceID,
		Name:           body.Name,
		PushPerMin:     int64(*body.PushPerMin),
		PushBurst:      int64(*body.PushBurst),
		PushDailyQuota: int64(*body.PushDailyQuota),
		CreatedAt:      time.Now().Unix(),
	}); err != nil {
		if db.IsUniqueViolation(err) {
			SendError(w, r, ErrorConflict().WithMessage("instance already has a scoped usage plan"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	p, err := h.queries.GetInstanceUsagePlan(r.Context(), id)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if h.onRateConfigChange != nil {
		h.onRateConfigChange(appID)
	}
	SendData(w, r, http.StatusCreated, toInstanceUsagePlanView(p))
}

func (h *ManagementHandler) listInstanceUsagePlans(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	rows, err := h.queries.ListInstanceUsagePlans(r.Context(), appID)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	out := make([]instanceUsagePlanView, 0, len(rows))
	for _, p := range rows {
		out = append(out, toInstanceUsagePlanView(p))
	}
	SendData(w, r, http.StatusOK, out)
}

func (h *ManagementHandler) updateInstanceUsagePlan(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	planID := r.PathValue("plan_id")
	var body struct {
		Name           *string `json:"name"`
		PushPerMin     *int    `json:"push_per_min"`
		PushBurst      *int    `json:"push_burst"`
		PushDailyQuota *int    `json:"push_daily_quota"`
	}
	if err := ReadRequestBodyJSON(w, r, &body); err != nil {
		SendError(w, r, err)
		return
	}
	if e := requireNonNegative(
		namedLimit{"push_per_min", body.PushPerMin},
		namedLimit{"push_burst", body.PushBurst},
		namedLimit{"push_daily_quota", body.PushDailyQuota},
	); e != nil {
		SendError(w, r, e)
		return
	}
	existing, err := h.queries.GetInstanceUsagePlan(r.Context(), planID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound())
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if existing.AppID != appID {
		// 404 rather than 403 to match the read path (don't leak the plan's
		// existence across apps).
		SendError(w, r, ErrorNotFound())
		return
	}
	updateParams := db.UpdateInstanceUsagePlanParams{
		ID:             planID,
		PushPerMin:     db.NewNullInt64(body.PushPerMin),
		PushBurst:      db.NewNullInt64(body.PushBurst),
		PushDailyQuota: db.NewNullInt64(body.PushDailyQuota),
	}
	updateParams.Name = db.NewNullString(body.Name)
	n, err := h.queries.UpdateInstanceUsagePlan(r.Context(), updateParams)
	if err != nil {
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if n == 0 {
		SendError(w, r, ErrorNotFound())
		return
	}
	if h.onRateConfigChange != nil {
		h.onRateConfigChange(appID)
	}
	SendData(w, r, http.StatusNoContent, nil)
}

func (h *ManagementHandler) deleteInstanceUsagePlan(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	planID := r.PathValue("plan_id")
	// Fetch first so we can distinguish not-found, wrong-app, and the
	// undeletable default with precise statuses.
	p, err := h.queries.GetInstanceUsagePlan(r.Context(), planID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound().WithMessage("instance usage plan not found"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if p.AppID != appID {
		SendError(w, r, ErrorNotFound().WithMessage("instance usage plan not found"))
		return
	}
	if p.IsDefault {
		SendError(w, r, ErrorForbidden().WithMessage("the default instance plan cannot be deleted"))
		return
	}
	n, err := h.queries.DeleteInstanceUsagePlan(r.Context(), db.DeleteInstanceUsagePlanParams{
		ID:    planID,
		AppID: appID,
	})
	if err != nil {
		if db.IsForeignKeyViolation(err) {
			SendError(w, r, ErrorConflict().WithMessage("plan is in use by one or more instances"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if n == 0 {
		SendError(w, r, ErrorNotFound().WithMessage("instance usage plan not found"))
		return
	}
	if h.onRateConfigChange != nil {
		h.onRateConfigChange(appID)
	}
	SendData(w, r, http.StatusNoContent, nil)
}

// getAssignedInstanceUsagePlan returns the assigned plan, or the app's default
// instance plan when none is assigned (the resolver's fallback order).
func (h *ManagementHandler) getAssignedInstanceUsagePlan(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	iid := r.PathValue("instance_id")
	ctx := r.Context()
	inst, err := h.queries.GetInstance(ctx, iid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound())
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	if inst.AppID != appID {
		SendError(w, r, ErrorForbidden())
		return
	}
	var p db.InstanceUsagePlan
	if inst.UsagePlanID.Valid {
		p, err = h.queries.GetInstanceUsagePlan(ctx, inst.UsagePlanID.String)
	} else {
		p, err = h.queries.GetDefaultInstanceUsagePlan(ctx, appID)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SendError(w, r, ErrorNotFound().WithMessage("no usage plan assigned"))
			return
		}
		SendError(w, r, ErrorInternalServerError().WithCause(err))
		return
	}
	SendData(w, r, http.StatusOK, toInstanceUsagePlanView(p))
}

func newPlanID() string { return id.New() }
