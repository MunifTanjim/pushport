package ratelimit

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/MunifTanjim/pushport/internal/db"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func must2[T any](_ T, err error) error { return err }

// nzi wraps an int64 as a "set this column" nullable update param.
func nzi(n int64) db.NullInt64 { return db.NewNullInt64(n) }

// setDefaultInstancePlan upserts the app's default instance plan, mirroring what
// app.Service.Create seeds and the owner then tunes.
func setDefaultInstancePlan(t *testing.T, d *db.DB, appID, planID string, perMin, burst, quota int64) {
	t.Helper()
	ctx := context.Background()
	if existing, err := d.Queries.GetDefaultInstanceUsagePlan(ctx, appID); err == nil {
		must(t, must2(d.Queries.UpdateInstanceUsagePlan(ctx, db.UpdateInstanceUsagePlanParams{
			ID:             existing.ID,
			PushPerMin:     nzi(perMin),
			PushBurst:      nzi(burst),
			PushDailyQuota: nzi(quota),
		})))
		return
	}
	must(t, d.Queries.CreateDefaultInstanceUsagePlan(ctx, db.CreateDefaultInstanceUsagePlanParams{
		ID: planID, AppID: appID, Name: "default",
		PushPerMin: perMin, PushBurst: burst, PushDailyQuota: quota, CreatedAt: 1,
	}))
}

func seedGlobalAppDefault(t *testing.T, d *db.DB, perMin, dailyQuota int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := d.Queries.GetDefaultAppUsagePlan(ctx); err == nil {
		return
	}
	must(t, d.Queries.CreateDefaultAppUsagePlan(ctx, db.CreateDefaultAppUsagePlanParams{
		ID: "global-default-app", Name: "default",
		PushPerMin: perMin, PushDailyQuota: dailyQuota, CreatedAt: 1,
	}))
}

func seedDefaults(t *testing.T, d *db.DB, appID string, def Limits) {
	t.Helper()
	seedGlobalAppDefault(t, d, int64(def.AppPushPerMin), int64(def.AppPushDailyQuota))
	setDefaultInstancePlan(t, d, appID, appID+"-inst-default",
		int64(def.InstancePushPerMin), int64(def.InstancePushBurst), int64(def.InstancePushDailyQuota))
}

func newResolverDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Queries.CreateApp(context.Background(), db.CreateAppParams{ID: "a", Name: "test"}); err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	return d
}

func TestResolverNoAssignedPlansUsesDefaultRows(t *testing.T) {
	d := newResolverDB(t)
	ctx := context.Background()
	defaults := Limits{InstancePushPerMin: 10, InstancePushBurst: 2, AppPushPerMin: 100, AppPushDailyQuota: 500}
	seedDefaults(t, d, "a", defaults)
	r := NewResolver(d.Queries)
	got := r.Limits(ctx, "a", "")
	if got != defaults {
		t.Fatalf("want defaults %+v, got %+v", defaults, got)
	}
}

func TestResolverAppPlanFullOverride(t *testing.T) {
	d := newResolverDB(t)
	ctx := context.Background()
	seedDefaults(t, d, "a", Limits{AppPushPerMin: 10, AppPushDailyQuota: 99})
	must(t, d.Queries.CreateAppUsagePlan(ctx, db.CreateAppUsagePlanParams{
		ID: "ap1", Name: "shared", PushDailyQuota: 42, CreatedAt: 1,
	}))
	must(t, must2(d.Queries.AssignAppUsagePlan(ctx, db.AssignAppUsagePlanParams{
		ID: "a", UsagePlanID: db.NewNullString("ap1"),
	})))
	r := NewResolver(d.Queries)
	got := r.Limits(ctx, "a", "")
	if got.AppPushPerMin != 0 {
		t.Fatalf("want AppPushPerMin=0 (assigned plan overwrites default with unlimited), got %d", got.AppPushPerMin)
	}
	if got.AppPushDailyQuota != 42 {
		t.Fatalf("want AppPushDailyQuota=42 (from assigned plan), got %d", got.AppPushDailyQuota)
	}
}

func TestResolverAppDefaultInstancePlan(t *testing.T) {
	d := newResolverDB(t)
	ctx := context.Background()
	setDefaultInstancePlan(t, d, "a", "idef", 55, 7, 0)
	r := NewResolver(d.Queries)
	got := r.Limits(ctx, "a", "")
	if got.InstancePushPerMin != 55 || got.InstancePushBurst != 7 {
		t.Fatalf("want per-min 55 burst 7 from app default, got %+v", got)
	}
}

func TestResolverInstancePlanFullOverride(t *testing.T) {
	d := newResolverDB(t)
	ctx := context.Background()
	setDefaultInstancePlan(t, d, "a", "def", 100, 10, 0)
	must(t, d.Queries.CreateInstanceUsagePlan(ctx, db.CreateInstanceUsagePlanParams{
		ID: "ovr", AppID: "a", Name: "shared", PushBurst: 3, CreatedAt: 1,
	}))
	r := NewResolver(d.Queries)
	got := r.Limits(ctx, "a", "ovr")
	if got.InstancePushPerMin != 0 || got.InstancePushBurst != 3 {
		t.Fatalf("want per-min 0 (overwritten with unlimited) burst 3 (from override), got %+v", got)
	}
}

func TestResolverInstanceDailyQuota(t *testing.T) {
	d := newResolverDB(t)
	ctx := context.Background()

	setDefaultInstancePlan(t, d, "a", "idef", 0, 0, 50)
	must(t, d.Queries.CreateInstanceUsagePlan(ctx, db.CreateInstanceUsagePlanParams{
		ID: "iovr", AppID: "a", Name: "shared", PushDailyQuota: 9, CreatedAt: 1,
	}))
	r := NewResolver(d.Queries)
	if got := r.Limits(ctx, "a", ""); got.InstancePushDailyQuota != 50 {
		t.Fatalf("want 50 from app default, got %d", got.InstancePushDailyQuota)
	}
	if got := r.Limits(ctx, "a", "iovr"); got.InstancePushDailyQuota != 9 {
		t.Fatalf("want 9 from instance plan, got %d", got.InstancePushDailyQuota)
	}
}

func TestResolverMissingInstancePlanFallsBack(t *testing.T) {
	d := newResolverDB(t)
	ctx := context.Background()
	defaults := Limits{InstancePushPerMin: 20, InstancePushBurst: 3}
	seedDefaults(t, d, "a", defaults)
	r := NewResolver(d.Queries)
	got := r.Limits(ctx, "a", "nonexistent-plan-id")
	if got.InstancePushPerMin != 20 || got.InstancePushBurst != 3 {
		t.Fatalf("want defaults on missing plan, got %+v", got)
	}
}

func TestResolverGetAppErrorNotCached(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()
	r := NewResolver(d.Queries)

	// App "x" does not exist yet — GetApp errors → fail open, must not be cached.
	got1 := r.Limits(ctx, "x", "")
	if got1 != (Limits{}) {
		t.Fatalf("before create: want empty limits (fail open), got %+v", got1)
	}

	if err := d.Queries.CreateApp(ctx, db.CreateAppParams{ID: "x", Name: "x"}); err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	must(t, d.Queries.CreateAppUsagePlan(ctx, db.CreateAppUsagePlanParams{
		ID: "ap-x", Name: "shared", PushPerMin: 77, PushDailyQuota: 999, CreatedAt: 1,
	}))
	must(t, must2(d.Queries.AssignAppUsagePlan(ctx, db.AssignAppUsagePlanParams{
		ID: "x", UsagePlanID: db.NewNullString("ap-x"),
	})))

	// Without Flush, a second call must now reflect the plan (proving the error was not cached).
	got2 := r.Limits(ctx, "x", "")
	if got2.AppPushPerMin != 77 {
		t.Fatalf("after create (no flush): want AppPushPerMin=77, got %d (error was cached)", got2.AppPushPerMin)
	}
	if got2.AppPushDailyQuota != 999 {
		t.Fatalf("after create (no flush): want AppPushDailyQuota=999, got %d", got2.AppPushDailyQuota)
	}
}

func TestResolverFlushClearsCache(t *testing.T) {
	d := newResolverDB(t)
	ctx := context.Background()
	must(t, d.Queries.CreateAppUsagePlan(ctx, db.CreateAppUsagePlanParams{
		ID: "ap-flush", Name: "shared", PushPerMin: 10, CreatedAt: 1,
	}))
	must(t, must2(d.Queries.AssignAppUsagePlan(ctx, db.AssignAppUsagePlanParams{
		ID: "a", UsagePlanID: db.NewNullString("ap-flush"),
	})))
	r := NewResolver(d.Queries)

	got1 := r.Limits(ctx, "a", "")
	if got1.AppPushPerMin != 10 {
		t.Fatalf("before flush: want 10, got %d", got1.AppPushPerMin)
	}

	must(t, must2(d.Queries.UpdateAppUsagePlan(ctx, db.UpdateAppUsagePlanParams{
		ID:         "ap-flush",
		PushPerMin: nzi(99),
	})))

	got2 := r.Limits(ctx, "a", "")
	if got2.AppPushPerMin != 10 {
		t.Fatalf("without flush: want 10 (cached), got %d", got2.AppPushPerMin)
	}

	r.Flush()
	got3 := r.Limits(ctx, "a", "")
	if got3.AppPushPerMin != 99 {
		t.Fatalf("after flush: want 99, got %d", got3.AppPushPerMin)
	}
}

type stubPlanReader struct {
	app            db.App
	defaultAppPlan db.AppUsagePlan
}

func (s *stubPlanReader) GetApp(_ context.Context, _ string) (db.App, error) {
	return s.app, nil
}

func (s *stubPlanReader) GetDefaultAppUsagePlan(_ context.Context) (db.AppUsagePlan, error) {
	return s.defaultAppPlan, nil
}

func (s *stubPlanReader) GetAppUsagePlan(_ context.Context, _ string) (db.AppUsagePlan, error) {
	return db.AppUsagePlan{}, sql.ErrNoRows
}

func (s *stubPlanReader) GetDefaultInstanceUsagePlan(_ context.Context, _ string) (db.InstanceUsagePlan, error) {
	return db.InstanceUsagePlan{}, sql.ErrNoRows
}

func (s *stubPlanReader) GetInstanceUsagePlan(_ context.Context, _ string) (db.InstanceUsagePlan, error) {
	return db.InstanceUsagePlan{}, sql.ErrNoRows
}

func TestResolverAppSelfRegAndSubscribe(t *testing.T) {
	stub := &stubPlanReader{
		app: db.App{ID: "a1"},
		defaultAppPlan: db.AppUsagePlan{
			IsDefault:      true,
			RegisterPerMin: 10, RegisterBurst: 5,
			RegisterIpPerMin: 5, RegisterIpBurst: 3,
			SubscribePerMin: 30, SubscribeBurst: 15,
		},
	}
	r := NewResolver(stub)
	got := r.Limits(context.Background(), "a1", "")
	if got.AppRegisterPerMin != 10 || got.AppRegisterBurst != 5 ||
		got.AppRegisterIPPerMin != 5 || got.AppRegisterIPBurst != 3 ||
		got.AppSubscribePerMin != 30 || got.AppSubscribeBurst != 15 {
		t.Fatalf("wrong limits: %+v", got)
	}
}

type stubQueries struct {
	*db.Queries
	failAppPlanRead bool
}

func (s *stubQueries) GetAppUsagePlan(ctx context.Context, id string) (db.AppUsagePlan, error) {
	if s.failAppPlanRead {
		return db.AppUsagePlan{}, errors.New("plan read failed")
	}
	return s.Queries.GetAppUsagePlan(ctx, id)
}

func TestResolverFailoverUsesLastGood(t *testing.T) {
	d := newResolverDB(t)
	ctx := context.Background()
	must(t, d.Queries.CreateAppUsagePlan(ctx, db.CreateAppUsagePlanParams{
		ID: "ap1", Name: "shared", PushPerMin: 77, PushDailyQuota: 999, CreatedAt: 1,
	}))
	must(t, must2(d.Queries.AssignAppUsagePlan(ctx, db.AssignAppUsagePlanParams{
		ID: "a", UsagePlanID: db.NewNullString("ap1"),
	})))
	sq := &stubQueries{Queries: d.Queries}
	r := NewResolver(sq)

	// Prime last-known-good with a successful resolve.
	if got := r.Limits(ctx, "a", ""); got.AppPushPerMin != 77 {
		t.Fatalf("prime: want 77, got %d", got.AppPushPerMin)
	}

	// Simulate a plan change (clears the active cache) followed by a DB error.
	r.Flush()
	sq.failAppPlanRead = true

	// Enforcement must survive via last-known-good, not fail open to empty.
	if got := r.Limits(ctx, "a", ""); got.AppPushPerMin != 77 || got.AppPushDailyQuota != 999 {
		t.Fatalf("want last-known-good {77,999} on DB error, got %+v", got)
	}

	// A never-seen key with no prior success still fails open (empty).
	if got := r.Limits(ctx, "b", ""); got != (Limits{}) {
		t.Fatalf("cold-cache error: want empty (fail open), got %+v", got)
	}
}

func TestResolverPlanReadErrorNotCached(t *testing.T) {
	d := newResolverDB(t)
	ctx := context.Background()
	must(t, d.Queries.CreateAppUsagePlan(ctx, db.CreateAppUsagePlanParams{
		ID: "ap1", Name: "shared", PushPerMin: 77, PushDailyQuota: 999, CreatedAt: 1,
	}))
	must(t, must2(d.Queries.AssignAppUsagePlan(ctx, db.AssignAppUsagePlanParams{
		ID: "a", UsagePlanID: db.NewNullString("ap1"),
	})))

	sq := &stubQueries{Queries: d.Queries, failAppPlanRead: true}
	r := NewResolver(sq)

	// GetApp succeeds but the plan read fails → empty limits, not cached.
	if got := r.Limits(ctx, "a", ""); got != (Limits{}) {
		t.Fatalf("want empty limits on plan-read error, got %+v", got)
	}

	// The read heals → the next call resolves real values without Flush.
	sq.failAppPlanRead = false
	if got := r.Limits(ctx, "a", ""); got.AppPushPerMin != 77 || got.AppPushDailyQuota != 999 {
		t.Fatalf("want plan values after read heals, got %+v", got)
	}
}
