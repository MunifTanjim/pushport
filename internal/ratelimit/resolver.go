package ratelimit

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"github.com/MunifTanjim/pushport/internal/db"
)

// Limits holds effective, fully-resolved rate-limit parameters for a request.
type Limits struct {
	InstancePushPerMin     int
	InstancePushBurst      int
	InstancePushDailyQuota int
	AppPushPerMin          int
	AppPushDailyQuota      int
	AppRegisterPerMin      int
	AppRegisterBurst       int
	AppRegisterIPPerMin    int
	AppRegisterIPBurst     int
	AppSubscribePerMin     int
	AppSubscribeBurst      int
}

// An assigned plan overwrites every column of the default it overlays; a 0
// column means unlimited (there is no NULL/inherit).
//
// planReader is the slice of the query API the Resolver needs (interface for
// test stubbing).
type planReader interface {
	GetApp(ctx context.Context, appID string) (db.App, error)
	GetDefaultAppUsagePlan(ctx context.Context) (db.AppUsagePlan, error)
	GetAppUsagePlan(ctx context.Context, id string) (db.AppUsagePlan, error)
	GetDefaultInstanceUsagePlan(ctx context.Context, appID string) (db.InstanceUsagePlan, error)
	GetInstanceUsagePlan(ctx context.Context, id string) (db.InstanceUsagePlan, error)
}

type Resolver struct {
	q planReader

	mu sync.Mutex
	// cache holds authoritative resolved limits; cleared by Flush on plan changes.
	cache map[string]Limits
	// lastGood retains the last successfully-resolved limits per key and survives
	// Flush, so a DB error can fall back to recent values instead of failing open.
	lastGood map[string]Limits
}

func NewResolver(q planReader) *Resolver {
	return &Resolver{q: q, cache: make(map[string]Limits), lastGood: make(map[string]Limits)}
}

func (r *Resolver) Limits(ctx context.Context, appID, instancePlanID string) Limits {
	key := appID + "\x00" + instancePlanID
	r.mu.Lock()
	if lim, ok := r.cache[key]; ok {
		r.mu.Unlock()
		return lim
	}
	r.mu.Unlock()

	lim, ok := r.resolve(ctx, appID, instancePlanID)
	if ok {
		r.mu.Lock()
		r.cache[key] = lim
		r.lastGood[key] = lim
		r.mu.Unlock()
		return lim
	}

	// resolve failed: fall back to the last successfully-resolved limits; a key
	// with no prior success falls through to empty (fail open).
	r.mu.Lock()
	lg, ok := r.lastGood[key]
	r.mu.Unlock()
	if ok {
		return lg
	}
	return lim
}

// resolve returns ok=false on a GetApp or plan-read failure; the caller must
// not cache the empty result (fail open). Missing plan rows (ErrNoRows) are
// skipped, not treated as failures.
func (r *Resolver) resolve(ctx context.Context, appID, instancePlanID string) (Limits, bool) {
	var lim Limits
	app, err := r.q.GetApp(ctx, appID)
	if err != nil {
		return Limits{}, false
	}
	// App plan: global default app plan, then the app's assigned plan overlays it.
	if p, err := r.q.GetDefaultAppUsagePlan(ctx); err == nil {
		applyAppPlan(p, &lim)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Limits{}, false
	}
	if app.UsagePlanID.Valid {
		if p, err := r.q.GetAppUsagePlan(ctx, app.UsagePlanID.String); err == nil {
			applyAppPlan(p, &lim)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return Limits{}, false
		}
	}
	// Instance plan: the app's default instance plan, then the instance's own plan.
	if p, err := r.q.GetDefaultInstanceUsagePlan(ctx, appID); err == nil {
		applyInstancePlan(p, &lim)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Limits{}, false
	}
	if instancePlanID != "" {
		if p, err := r.q.GetInstanceUsagePlan(ctx, instancePlanID); err == nil {
			applyInstancePlan(p, &lim)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return Limits{}, false
		}
	}
	return lim, true
}

func applyAppPlan(p db.AppUsagePlan, lim *Limits) {
	lim.AppPushPerMin = int(p.PushPerMin)
	lim.AppPushDailyQuota = int(p.PushDailyQuota)
	lim.AppRegisterPerMin = int(p.RegisterPerMin)
	lim.AppRegisterBurst = int(p.RegisterBurst)
	lim.AppRegisterIPPerMin = int(p.RegisterIpPerMin)
	lim.AppRegisterIPBurst = int(p.RegisterIpBurst)
	lim.AppSubscribePerMin = int(p.SubscribePerMin)
	lim.AppSubscribeBurst = int(p.SubscribeBurst)
}

func applyInstancePlan(p db.InstanceUsagePlan, lim *Limits) {
	lim.InstancePushPerMin = int(p.PushPerMin)
	lim.InstancePushBurst = int(p.PushBurst)
	lim.InstancePushDailyQuota = int(p.PushDailyQuota)
}

// Flush clears the authoritative cache. Call after any plan create/update/delete
// or assignment change; the next request re-resolves lazily. lastGood is retained
// so a DB error immediately after a change still falls back to recent values.
func (r *Resolver) Flush() {
	r.mu.Lock()
	r.cache = make(map[string]Limits)
	r.mu.Unlock()
}
