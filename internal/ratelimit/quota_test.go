package ratelimit

import (
	"testing"
	"time"
)

func TestQuotaEnforcesDailyLimit(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	q := NewQuota(func() time.Time { return now })
	if ok, _ := q.Allow("t1", 2); !ok {
		t.Fatal("1st allowed")
	}
	if ok, _ := q.Allow("t1", 2); !ok {
		t.Fatal("2nd allowed")
	}
	ok, retry := q.Allow("t1", 2)
	if ok {
		t.Fatal("3rd should be denied")
	}
	if retry <= 0 || retry > 24*time.Hour {
		t.Fatalf("retryAfter = %v, want until midnight", retry)
	}
	if ok, _ := q.Allow("t2", 2); !ok {
		t.Fatal("other tenant allowed")
	}
}

func TestQuotaResetsNextDay(t *testing.T) {
	now := time.Date(2026, 8, 28, 23, 0, 0, 0, time.UTC)
	q := NewQuota(func() time.Time { return now })
	if ok, _ := q.Allow("t1", 1); !ok {
		t.Fatal("allowed today")
	}
	if ok, _ := q.Allow("t1", 1); ok {
		t.Fatal("denied today")
	}
	now = now.Add(2 * time.Hour) // crosses UTC midnight
	if ok, _ := q.Allow("t1", 1); !ok {
		t.Fatal("allowed next day")
	}
}

func TestQuotaDisabled(t *testing.T) {
	q := NewQuota(nil)
	for i := 0; i < 100; i++ {
		if ok, _ := q.Allow("t1", 0); !ok {
			t.Fatal("disabled quota should always allow")
		}
	}
}

func TestQuotaAdmitAllOrNothing(t *testing.T) {
	now := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)
	q := NewQuota(func() time.Time { return now })
	if ok, _, _ := q.Admit(quotaCheck{"a", 1}, quotaCheck{"b", 5}); !ok {
		t.Fatal("first admit should pass")
	}
	ok, _, denied := q.Admit(quotaCheck{"a", 1}, quotaCheck{"b", 5})
	if ok || denied != "a" {
		t.Fatalf("want denial on a, got ok=%v denied=%q", ok, denied)
	}
	// "b" still has 4 slots left (only the first admit touched it).
	for i := 0; i < 4; i++ {
		if ok, _, _ := q.Admit(quotaCheck{"b", 5}); !ok {
			t.Fatalf("b slot %d should be free", i)
		}
	}
	if ok, _, denied := q.Admit(quotaCheck{"b", 5}); ok || denied != "b" {
		t.Fatalf("b should now be exhausted, got ok=%v denied=%q", ok, denied)
	}
}

func TestQuotaSnapshotAndLoadRoundTrip(t *testing.T) {
	day := "2026-08-31"
	now := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)
	q := NewQuota(func() time.Time { return now })
	q.Allow("t1", 10)
	q.Allow("t1", 10)
	q.Allow("t2", 10)

	rows := q.Snapshot()
	got := map[string]int{}
	for _, r := range rows {
		if r.Day != day {
			t.Fatalf("row day = %q, want %q", r.Day, day)
		}
		got[r.Tenant] = r.Count
	}
	if got["t1"] != 2 || got["t2"] != 1 {
		t.Fatalf("snapshot counts = %v, want t1=2 t2=1", got)
	}
	if rows := q.Snapshot(); len(rows) != 0 {
		t.Fatalf("second snapshot should be empty, got %v", rows)
	}

	q2 := NewQuota(func() time.Time { return now })
	q2.Load(day, got)
	if ok, _ := q2.Allow("t1", 2); ok {
		t.Fatal("t1 already at limit 2 after load, should be denied")
	}
	if ok, _ := q2.Allow("t2", 2); !ok {
		t.Fatal("t2 at 1/2 after load, should allow one more")
	}
}

func TestQuotaSnapshotCapturesOutgoingDayOnRollover(t *testing.T) {
	now := time.Date(2026, 8, 31, 23, 0, 0, 0, time.UTC)
	q := NewQuota(func() time.Time { return now })
	q.Allow("t1", 10) // counts for 2026-08-31

	now = now.Add(2 * time.Hour) // → 2026-09-01, triggers rollover on next admit
	q.Allow("t1", 10)            // counts for 2026-09-01

	byDay := map[string]int{}
	for _, r := range q.Snapshot() {
		byDay[r.Day] += r.Count
	}
	if byDay["2026-08-31"] != 1 {
		t.Fatalf("outgoing day count = %d, want 1", byDay["2026-08-31"])
	}
	if byDay["2026-09-01"] != 1 {
		t.Fatalf("new day count = %d, want 1", byDay["2026-09-01"])
	}
}
