package ratelimit

import (
	"context"
	"testing"

	"github.com/MunifTanjim/pushport/internal/db"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func insertApp(t *testing.T, q *db.Queries, id string) {
	t.Helper()
	err := q.CreateApp(context.Background(), db.CreateAppParams{
		ID: id, Name: "test",
	})
	if err != nil {
		t.Fatalf("insertApp: %v", err)
	}
}

func TestGateBurstExhaustedDenies(t *testing.T) {
	d := newTestDB(t)
	insertApp(t, d.Queries, "t1")
	defaults := Limits{InstancePushPerMin: 60, InstancePushBurst: 1}
	seedDefaults(t, d, "t1", defaults)
	resolver := NewResolver(d.Queries)
	gate := NewGate(resolver)
	ctx := context.Background()

	if ok, _, _ := gate.Admit(ctx, "t1", "k", ""); !ok {
		t.Fatal("first admit should pass")
	}
	ok, retry, reason := gate.Admit(ctx, "t1", "k", "")
	if ok || reason != "instance_rate" || retry <= 0 {
		t.Fatalf("want instance_rate denial, got ok=%v reason=%q retry=%v", ok, reason, retry)
	}
}

func TestGateAppRateDeniesWithDerivedBurst(t *testing.T) {
	d := newTestDB(t)
	insertApp(t, d.Queries, "tapp")
	defaults := Limits{AppPushPerMin: 2}
	seedDefaults(t, d, "tapp", defaults)
	resolver := NewResolver(d.Queries)
	gate := NewGate(resolver)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if ok, _, reason := gate.Admit(ctx, "tapp", "k", ""); !ok {
			t.Fatalf("app admit %d should pass, got reason=%q", i, reason)
		}
	}
	ok, retry, reason := gate.Admit(ctx, "tapp", "k", "")
	if ok || reason != "app_rate" || retry <= 0 {
		t.Fatalf("want app_rate denial, got ok=%v reason=%q retry=%v", ok, reason, retry)
	}
}

func TestGateQuotaDenies(t *testing.T) {
	d := newTestDB(t)
	insertApp(t, d.Queries, "tq")
	defaults := Limits{AppPushDailyQuota: 1}
	seedDefaults(t, d, "tq", defaults)
	resolver := NewResolver(d.Queries)
	gate := NewGate(resolver)
	ctx := context.Background()

	if ok, _, _ := gate.Admit(ctx, "tq", "k", ""); !ok {
		t.Fatal("first admit should pass")
	}
	ok, _, reason := gate.Admit(ctx, "tq", "k", "")
	if ok || reason != "app_quota" {
		t.Fatalf("want app_quota denial, got ok=%v reason=%q", ok, reason)
	}
}

func TestGateInstanceQuotaDenies(t *testing.T) {
	d := newTestDB(t)
	insertApp(t, d.Queries, "tiq")
	defaults := Limits{InstancePushDailyQuota: 1}
	seedDefaults(t, d, "tiq", defaults)
	resolver := NewResolver(d.Queries)
	gate := NewGate(resolver)
	ctx := context.Background()

	if ok, _, _ := gate.Admit(ctx, "tiq", "k", ""); !ok {
		t.Fatal("first admit should pass")
	}
	if ok, _, reason := gate.Admit(ctx, "tiq", "k", ""); ok || reason != "instance_quota" {
		t.Fatalf("want instance_quota denial, got ok=%v reason=%q", ok, reason)
	}
	if ok, _, reason := gate.Admit(ctx, "tiq", "k2", ""); !ok {
		t.Fatalf("other instance should pass, reason=%q", reason)
	}
}

func TestGateQuotaAllOrNothing(t *testing.T) {
	d := newTestDB(t)
	insertApp(t, d.Queries, "tan")
	defaults := Limits{AppPushDailyQuota: 1, InstancePushDailyQuota: 5}
	seedDefaults(t, d, "tan", defaults)
	resolver := NewResolver(d.Queries)
	gate := NewGate(resolver)
	ctx := context.Background()

	if ok, _, _ := gate.Admit(ctx, "tan", "k", ""); !ok {
		t.Fatal("first admit should pass")
	}
	// Commit is all-or-nothing: this app-quota denial must NOT consume k2's instance quota.
	if ok, _, reason := gate.Admit(ctx, "tan", "k2", ""); ok || reason != "app_quota" {
		t.Fatalf("want app_quota denial, got ok=%v reason=%q", ok, reason)
	}
	// Prove k2's instance quota is untouched: raise the app daily quota (its
	// instance quota stays 5) and re-resolve; k2 should still have all 5 slots.
	defApp, err := d.Queries.GetDefaultAppUsagePlan(ctx)
	must(t, err)
	must(t, must2(d.Queries.UpdateAppUsagePlan(ctx, db.UpdateAppUsagePlanParams{
		ID: defApp.ID, PushDailyQuota: nzi(100),
	})))
	resolver.Flush()
	for i := 0; i < 5; i++ {
		if ok, _, reason := gate.Admit(ctx, "tan", "k2", ""); !ok {
			t.Fatalf("k2 instance slot %d should be free (not consumed by denied push), reason=%q", i, reason)
		}
	}
}

func TestGateAllLimitsDisabledAllows(t *testing.T) {
	d := newTestDB(t)
	insertApp(t, d.Queries, "tall")
	resolver := NewResolver(d.Queries)
	gate := NewGate(resolver)
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		if ok, _, _ := gate.Admit(ctx, "tall", "k", ""); !ok {
			t.Fatalf("all-disabled gate should always allow (i=%d)", i)
		}
	}
}

func TestGatePerTenantOverrideChangesLimit(t *testing.T) {
	d := newTestDB(t)
	insertApp(t, d.Queries, "tov")
	defaults := Limits{InstancePushBurst: 1}
	seedDefaults(t, d, "tov", defaults)
	resolver := NewResolver(d.Queries)
	gate := NewGate(resolver)
	ctx := context.Background()

	if ok, _, _ := gate.Admit(ctx, "tov", "k", ""); !ok {
		t.Fatal("first admit should pass (instance rate disabled by default)")
	}

	setDefaultInstancePlan(t, d, "tov", "ov-def", 60, 1, 0)

	resolver.Flush()

	// Instance rate was disabled before, so "k" has no bucket yet; the first call
	// after the plan creates it (burst=1) and consumes its token.
	if ok, _, _ := gate.Admit(ctx, "tov", "k", ""); !ok {
		t.Fatal("first admit after override should pass")
	}
	ok, retry, reason := gate.Admit(ctx, "tov", "k", "")
	if ok || reason != "instance_rate" || retry <= 0 {
		t.Fatalf("want instance_rate denial after override, got ok=%v reason=%q retry=%v", ok, reason, retry)
	}
}

func TestGateRateDenialDoesNotConsumeQuota(t *testing.T) {
	d := newTestDB(t)
	insertApp(t, d.Queries, "trd")
	defaults := Limits{InstancePushPerMin: 60, InstancePushBurst: 1, AppPushDailyQuota: 2}
	seedDefaults(t, d, "trd", defaults)
	resolver := NewResolver(d.Queries)
	gate := NewGate(resolver)
	ctx := context.Background()

	if ok, _, _ := gate.Admit(ctx, "trd", "k", ""); !ok {
		t.Fatal("first admit should pass")
	}
	if ok, _, reason := gate.Admit(ctx, "trd", "k", ""); ok || reason != "instance_rate" {
		t.Fatalf("want instance_rate denial, got ok=%v reason=%q", ok, reason)
	}
	// A DIFFERENT instance gets a fresh rate bucket, so it clears the rate limit and
	// reaches the quota. The quota count is still 1 (the rate-denied request never
	// touched it), so with AppPushDailyQuota=2 this is allowed.
	if ok, _, reason := gate.Admit(ctx, "trd", "k2", ""); !ok {
		t.Fatalf("quota should not have been drained by the rate-denied request; denial reason=%q", reason)
	}
}

func TestGateAppRateDenialRefundsInstanceToken(t *testing.T) {
	d := newTestDB(t)
	insertApp(t, d.Queries, "tar")
	defaults := Limits{InstancePushPerMin: 60, InstancePushBurst: 2, AppPushPerMin: 1}
	seedDefaults(t, d, "tar", defaults)
	resolver := NewResolver(d.Queries)
	inst := NewLimiter(nil)
	app := NewLimiter(nil)
	gate := newGateWith(resolver, inst, app, NewQuota(nil))
	ctx := context.Background()

	if ok, _, _ := gate.Admit(ctx, "tar", "k", ""); !ok {
		t.Fatal("first admit should pass")
	}
	if ok, _, reason := gate.Admit(ctx, "tar", "k", ""); ok || reason != "app_rate" {
		t.Fatalf("want app_rate denial, got ok=%v reason=%q", ok, reason)
	}
	if !inst.Allowed("k", 60, 2) {
		t.Fatal("app_rate denial drained the instance token instead of refunding it")
	}
}

func TestGateQuotaDenialRefundsRateTokens(t *testing.T) {
	d := newTestDB(t)
	insertApp(t, d.Queries, "tqr")
	defaults := Limits{InstancePushPerMin: 60, InstancePushBurst: 2, AppPushDailyQuota: 1}
	seedDefaults(t, d, "tqr", defaults)
	resolver := NewResolver(d.Queries)
	inst := NewLimiter(nil)
	gate := newGateWith(resolver, inst, NewLimiter(nil), NewQuota(nil))
	ctx := context.Background()

	if ok, _, _ := gate.Admit(ctx, "tqr", "k", ""); !ok {
		t.Fatal("first admit should pass")
	}
	if ok, _, reason := gate.Admit(ctx, "tqr", "k", ""); ok || reason != "app_quota" {
		t.Fatalf("want app_quota denial, got ok=%v reason=%q", ok, reason)
	}
	if !inst.Allowed("k", 60, 2) {
		t.Fatal("quota denial drained the instance token instead of refunding it")
	}
}

func TestGateNegativeLimitsUnlimited(t *testing.T) {
	d := newTestDB(t)
	insertApp(t, d.Queries, "tneg")
	defaults := Limits{InstancePushPerMin: -1, AppPushPerMin: -1, InstancePushBurst: -1, AppPushDailyQuota: -1}
	seedDefaults(t, d, "tneg", defaults)
	resolver := NewResolver(d.Queries)
	gate := NewGate(resolver)
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		if ok, _, reason := gate.Admit(ctx, "tneg", "k", ""); !ok {
			t.Fatalf("negative limits should always allow (i=%d, reason=%q)", i, reason)
		}
	}
}

func TestGateInstancePlanOverrides(t *testing.T) {
	d := newTestDB(t)
	insertApp(t, d.Queries, "tpk")
	ctx := context.Background()
	setDefaultInstancePlan(t, d, "tpk", "def", 60, 1, 0)
	must(t, d.Queries.CreateInstanceUsagePlan(ctx, db.CreateInstanceUsagePlanParams{
		ID: "ovr", AppID: "tpk", Name: "shared", PushPerMin: 60, PushBurst: 5, CreatedAt: 1,
	}))
	resolver := NewResolver(d.Queries)
	gate := NewGate(resolver)

	if ok, _, _ := gate.Admit(ctx, "tpk", "k-default", ""); !ok {
		t.Fatal("first should pass")
	}
	if ok, _, reason := gate.Admit(ctx, "tpk", "k-default", ""); ok || reason != "instance_rate" {
		t.Fatalf("want instance_rate, got ok=%v reason=%q", ok, reason)
	}
	for i := 0; i < 5; i++ {
		if ok, _, reason := gate.Admit(ctx, "tpk", "k-ovr", "ovr"); !ok {
			t.Fatalf("override admit %d should pass, reason=%q", i, reason)
		}
	}
	if ok, _, reason := gate.Admit(ctx, "tpk", "k-ovr", "ovr"); ok || reason != "instance_rate" {
		t.Fatalf("want instance_rate after burst, got ok=%v reason=%q", ok, reason)
	}
}
