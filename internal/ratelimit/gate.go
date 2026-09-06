package ratelimit

import (
	"context"
	"strings"
	"time"
)

type Gate struct {
	resolver *Resolver
	quota    *Quota
	instance *Limiter
	app      *Limiter
}

func NewGate(resolver *Resolver) *Gate {
	return newGateWith(resolver, NewLimiter(nil), NewLimiter(nil), NewQuota(nil))
}

func newGateWith(resolver *Resolver, instance, app *Limiter, quota *Quota) *Gate {
	return &Gate{resolver: resolver, quota: quota, instance: instance, app: app}
}

// Admit checks limits in order — instance rate, app rate, then the instance and
// app daily quotas committed all-or-nothing — returning (false, retryAfter,
// reason) on the first denial. instancePlanID may be ""; any limit <= 0 means
// unlimited.
//
// Rate limits are reserved, not committed, so a denial at a later check rolls back
// tokens taken by earlier ones. Rollback via reservation cancel is best-effort
// under concurrency but always conservative: it can only leave a token spent,
// never admit one that should be blocked.
func (g *Gate) Admit(ctx context.Context, appID, instanceID, instancePlanID string) (bool, time.Duration, string) {
	lim := g.resolver.Limits(ctx, appID, instancePlanID)

	cancelInst, ok, retry := g.instance.Reserve(instanceID, lim.InstancePushPerMin, lim.InstancePushBurst)
	if !ok {
		return false, retry, "instance_rate"
	}
	// App rate: burst derived from its own per-min rate.
	cancelApp, ok, retry := g.app.Reserve(appID, lim.AppPushPerMin, lim.AppPushPerMin)
	if !ok {
		cancelInst()
		return false, retry, "app_rate"
	}
	if ok, retry, denied := g.quota.Admit(
		quotaCheck{tenant: ScopeInstance + instanceID, limit: lim.InstancePushDailyQuota},
		quotaCheck{tenant: ScopeApp + appID, limit: lim.AppPushDailyQuota},
	); !ok {
		// quota.Admit doesn't increment on denial, so only the rate tokens roll back
		cancelApp()
		cancelInst()
		reason := "app_quota"
		if strings.HasPrefix(denied, ScopeInstance) {
			reason = "instance_quota"
		}
		return false, retry, reason
	}
	return true, 0, ""
}

// Quota tenant-key prefixes namespace the shared usage_counter table. They are
// part of the on-disk scope format, so the stats endpoint reuses them to split
// counters back into per-app / per-instance counts.
const (
	ScopeApp      = "app:"
	ScopeInstance = "inst:"
)

func (g *Gate) Quota() *Quota { return g.quota }

// Cleanup evicts idle buckets from each limiter to bound memory.
func (g *Gate) Cleanup(idle time.Duration) {
	g.instance.Cleanup(idle)
	g.app.Cleanup(idle)
}
