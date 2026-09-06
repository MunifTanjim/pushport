package db

import (
	"context"
	"database/sql"
	"testing"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAppUsagePlanBespokeUnique(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()
	must(t, d.Queries.CreateApp(ctx, CreateAppParams{ID: "a1", Name: "n", CreatedAt: 1}))
	must(t, d.Queries.CreateAppUsagePlan(ctx, CreateAppUsagePlanParams{ID: "p1", AppID: NewNullString("a1"), CreatedAt: 1}))
	err = d.Queries.CreateAppUsagePlan(ctx, CreateAppUsagePlanParams{ID: "p2", AppID: NewNullString("a1"), CreatedAt: 2})
	if err == nil {
		t.Fatal("want unique-violation for second bespoke plan, got nil")
	}
}

func TestAppUsagePlanSharedNameUnique(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()
	must(t, d.Queries.CreateAppUsagePlan(ctx, CreateAppUsagePlanParams{ID: "s1", Name: "premium", CreatedAt: 1}))
	err = d.Queries.CreateAppUsagePlan(ctx, CreateAppUsagePlanParams{ID: "s2", Name: "premium", CreatedAt: 2})
	if err == nil {
		t.Fatal("want unique-violation for duplicate shared plan name, got nil")
	}
}

func TestInstanceUsagePlanCascadeOnDeleteInstance(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()

	must(t, d.Queries.CreateApp(ctx, CreateAppParams{ID: "a1", Name: "n", CreatedAt: 1}))
	must(t, d.Queries.CreateInstance(ctx, CreateInstanceParams{ID: "i1", AppID: "a1", Label: "srv", CreatedAt: 1}))
	must(t, d.Queries.CreateInstanceUsagePlan(ctx, CreateInstanceUsagePlanParams{
		ID:         "ip1",
		AppID:      "a1",
		InstanceID: NewNullString("i1"),
		CreatedAt:  1,
	}))

	plan, err := d.Queries.GetInstanceUsagePlan(ctx, "ip1")
	if err != nil {
		t.Fatalf("plan should exist before delete: %v", err)
	}
	if plan.ID != "ip1" {
		t.Fatalf("unexpected plan id: %s", plan.ID)
	}

	n, err := d.Queries.DeleteInstance(ctx, "i1")
	if err != nil || n != 1 {
		t.Fatalf("delete instance: n=%d err=%v", n, err)
	}

	if _, err := d.Queries.GetInstanceUsagePlan(ctx, "ip1"); !isNotFound(err) {
		t.Fatalf("want not-found after cascade delete, got %v", err)
	}
}

func TestAppUsagePlanRestrictDeleteWhileInUse(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()

	must(t, d.Queries.CreateApp(ctx, CreateAppParams{ID: "a1", Name: "n", CreatedAt: 1}))
	must(t, d.Queries.CreateAppUsagePlan(ctx, CreateAppUsagePlanParams{
		ID:        "sp1",
		Name:      "shared",
		CreatedAt: 1,
	}))
	n, err := d.Queries.AssignAppUsagePlan(ctx, AssignAppUsagePlanParams{
		UsagePlanID: NewNullString("sp1"),
		ID:          "a1",
	})
	if err != nil || n != 1 {
		t.Fatalf("assign plan: n=%d err=%v", n, err)
	}

	if _, err := d.Queries.DeleteAppUsagePlan(ctx, "sp1"); !IsForeignKeyViolation(err) {
		t.Fatalf("want foreign-key violation deleting in-use plan, got %v", err)
	}

	if _, err := d.Queries.AssignAppUsagePlan(ctx, AssignAppUsagePlanParams{ID: "a1"}); err != nil {
		t.Fatalf("unassign: %v", err)
	}
	n, err = d.Queries.DeleteAppUsagePlan(ctx, "sp1")
	if err != nil || n != 1 {
		t.Fatalf("delete after unassign: n=%d err=%v", n, err)
	}
}

func TestInstanceUsagePlanSharedNameUnique(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()
	must(t, d.Queries.CreateApp(ctx, CreateAppParams{ID: "a1", Name: "n", CreatedAt: 1}))
	must(t, d.Queries.CreateInstanceUsagePlan(ctx, CreateInstanceUsagePlanParams{
		ID: "ip1", AppID: "a1", Name: "fast", CreatedAt: 1,
	}))
	err = d.Queries.CreateInstanceUsagePlan(ctx, CreateInstanceUsagePlanParams{
		ID: "ip2", AppID: "a1", Name: "fast", CreatedAt: 2,
	})
	if err == nil {
		t.Fatal("want unique-violation for duplicate shared instance plan name, got nil")
	}
}

// TestUpdateAppUsagePlanThreeWay: a NULL param leaves the column unchanged, a
// present value sets it, and a present 0 clears it to unlimited.
func TestUpdateAppUsagePlanThreeWay(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()

	must(t, d.Queries.CreateAppUsagePlan(ctx, CreateAppUsagePlanParams{
		ID:         "p1",
		Name:       "base",
		PushPerMin: 60,
		CreatedAt:  1,
	}))

	n, err := d.Queries.UpdateAppUsagePlan(ctx, UpdateAppUsagePlanParams{
		ID:         "p1",
		PushPerMin: NullInt64{},
	})
	if err != nil || n != 1 {
		t.Fatalf("omit update: n=%d err=%v", n, err)
	}
	plan, err := d.Queries.GetAppUsagePlan(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if plan.PushPerMin != 60 {
		t.Fatalf("omit: want 60, got %d", plan.PushPerMin)
	}

	n, err = d.Queries.UpdateAppUsagePlan(ctx, UpdateAppUsagePlanParams{
		ID:         "p1",
		PushPerMin: NewNullInt64(120),
	})
	if err != nil || n != 1 {
		t.Fatalf("set update: n=%d err=%v", n, err)
	}
	plan, err = d.Queries.GetAppUsagePlan(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if plan.PushPerMin != 120 {
		t.Fatalf("set: want 120, got %d", plan.PushPerMin)
	}

	n, err = d.Queries.UpdateAppUsagePlan(ctx, UpdateAppUsagePlanParams{
		ID:         "p1",
		PushPerMin: NewNullInt64(0),
	})
	if err != nil || n != 1 {
		t.Fatalf("clear update: n=%d err=%v", n, err)
	}
	plan, err = d.Queries.GetAppUsagePlan(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if plan.PushPerMin != 0 {
		t.Fatalf("clear: want 0, got %d", plan.PushPerMin)
	}
}

func TestAppUsagePlanSelfRegSubscribeColumns(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()

	id := "plan_selfreg_1"
	if err := d.Queries.CreateAppUsagePlan(ctx, CreateAppUsagePlanParams{
		ID: id, Name: "p", PushPerMin: 0, PushDailyQuota: 0,
		RegisterPerMin: 10, RegisterBurst: 5,
		RegisterIpPerMin: 5, RegisterIpBurst: 3,
		SubscribePerMin: 30, SubscribeBurst: 15,
		CreatedAt: 1,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := d.Queries.GetAppUsagePlan(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.RegisterPerMin != 10 || got.RegisterBurst != 5 ||
		got.RegisterIpPerMin != 5 || got.RegisterIpBurst != 3 ||
		got.SubscribePerMin != 30 || got.SubscribeBurst != 15 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func isNotFound(err error) bool {
	return err == sql.ErrNoRows
}

// TestAppDeleteCascadesInstancesAndPlans: the ON DELETE NO ACTION plan references
// must tolerate parent and child being deleted together in one cascade.
func TestAppDeleteCascadesInstancesAndPlans(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()

	must(t, d.Queries.CreateApp(ctx, CreateAppParams{ID: "a1", Name: "n", CreatedAt: 1}))
	must(t, d.Queries.CreateInstance(ctx, CreateInstanceParams{ID: "i1", AppID: "a1", Label: "l", CreatedAt: 1}))
	must(t, d.Queries.CreateAppUsagePlan(ctx, CreateAppUsagePlanParams{
		ID: "ap1", AppID: NewNullString("a1"), CreatedAt: 1,
	}))
	if _, err := d.Queries.AssignAppUsagePlan(ctx, AssignAppUsagePlanParams{
		ID: "a1", UsagePlanID: NewNullString("ap1"),
	}); err != nil {
		t.Fatalf("assign app plan: %v", err)
	}
	must(t, d.Queries.CreateInstanceUsagePlan(ctx, CreateInstanceUsagePlanParams{ID: "ip1", AppID: "a1", CreatedAt: 1}))
	if _, err := d.Queries.AssignInstanceUsagePlan(ctx, AssignInstanceUsagePlanParams{
		ID: "i1", UsagePlanID: NewNullString("ip1"),
	}); err != nil {
		t.Fatalf("assign instance plan: %v", err)
	}

	// Delete the app row directly (no app-delete query exists yet).
	if _, err := d.sql.ExecContext(ctx, "DELETE FROM app WHERE id = ?", "a1"); err != nil {
		t.Fatalf("delete app: %v", err)
	}

	for _, q := range []struct {
		name string
		sql  string
	}{
		{"instance", "SELECT COUNT(*) FROM instance WHERE app_id='a1'"},
		{"app_usage_plan", "SELECT COUNT(*) FROM app_usage_plan WHERE app_id='a1'"},
		{"instance_usage_plan", "SELECT COUNT(*) FROM instance_usage_plan WHERE app_id='a1'"},
	} {
		var n int
		if err := d.sql.QueryRowContext(ctx, q.sql).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", q.name, err)
		}
		if n != 0 {
			t.Errorf("after app delete, %s rows = %d, want 0", q.name, n)
		}
	}
}

func TestInstanceUsagePlanRestrictDeleteWhileInUse(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()

	must(t, d.Queries.CreateApp(ctx, CreateAppParams{ID: "a1", Name: "n", CreatedAt: 1}))
	must(t, d.Queries.CreateInstance(ctx, CreateInstanceParams{ID: "i1", AppID: "a1", Label: "l", CreatedAt: 1}))
	must(t, d.Queries.CreateInstanceUsagePlan(ctx, CreateInstanceUsagePlanParams{ID: "ip1", AppID: "a1", CreatedAt: 1}))
	if _, err := d.Queries.AssignInstanceUsagePlan(ctx, AssignInstanceUsagePlanParams{
		ID: "i1", UsagePlanID: NewNullString("ip1"),
	}); err != nil {
		t.Fatalf("assign: %v", err)
	}

	if _, err := d.Queries.DeleteInstanceUsagePlan(ctx, DeleteInstanceUsagePlanParams{ID: "ip1", AppID: "a1"}); !IsForeignKeyViolation(err) {
		t.Fatalf("want foreign-key violation deleting in-use plan, got %v", err)
	}

	if _, err := d.Queries.AssignInstanceUsagePlan(ctx, AssignInstanceUsagePlanParams{ID: "i1"}); err != nil {
		t.Fatalf("unassign: %v", err)
	}
	n, err := d.Queries.DeleteInstanceUsagePlan(ctx, DeleteInstanceUsagePlanParams{ID: "ip1", AppID: "a1"})
	if err != nil || n != 1 {
		t.Fatalf("delete after unassign: n=%d err=%v", n, err)
	}
}
