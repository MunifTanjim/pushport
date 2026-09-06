package ratelimit

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/MunifTanjim/pushport/internal/db"
)

func TestQuotaStorePersistsAndRecovers(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	q1 := NewQuota(clock)
	q1.Allow("app:x", 100)
	q1.Allow("app:x", 100)
	q1.Allow("inst:y", 100)
	store1 := NewQuotaStore(q1, d.Queries, clock)
	if err := store1.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Simulated restart: a fresh Quota reloads today's counts.
	q2 := NewQuota(clock)
	store2 := NewQuotaStore(q2, d.Queries, clock)
	if err := store2.Load(ctx); err != nil {
		t.Fatalf("load: %v", err)
	}
	if ok, _ := q2.Allow("app:x", 2); ok {
		t.Fatal("app:x should be at limit 2 after recovery")
	}
	if ok, _ := q2.Allow("inst:y", 2); !ok {
		t.Fatal("inst:y should allow one more after recovery")
	}
	if ok, _ := q2.Allow("inst:y", 2); ok {
		t.Fatal("inst:y should now be exhausted")
	}
}

func TestQuotaStoreFlushIsIdempotent(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	q := NewQuota(clock)
	store := NewQuotaStore(q, d.Queries, clock)
	q.Allow("app:x", 100)
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush 1: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush 2: %v", err)
	}
	rows, err := d.Queries.ListUsageCountersForDay(ctx, dayDate("2026-08-31"))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].Scope != "app:x" || rows[0].Count != 1 {
		t.Fatalf("want single app:x=1 row, got %+v", rows)
	}
}

func TestQuotaStoreCleanupRemovesOldDays(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)

	oldClock := func() time.Time { return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) }
	qOld := NewQuota(oldClock)
	qOld.Allow("app:x", 100)
	if err := NewQuotaStore(qOld, d.Queries, oldClock).Flush(ctx); err != nil {
		t.Fatalf("flush old: %v", err)
	}

	store := NewQuotaStore(NewQuota(func() time.Time { return now }), d.Queries, func() time.Time { return now })
	if err := store.Cleanup(ctx); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if rows, err := d.Queries.ListUsageCountersForDay(ctx, dayDate("2026-08-29")); err != nil {
		t.Fatalf("list: %v", err)
	} else if len(rows) != 0 {
		t.Fatalf("old-day rows should be gone, got %+v", rows)
	}
}

func TestQuotaStoreFlushRetriesAfterError(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	path := filepath.Join(t.TempDir(), "quota.db")
	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	q := NewQuota(clock)
	q.Allow("app:x", 100)
	q.Allow("app:x", 100)
	store := NewQuotaStore(q, d.Queries, clock)

	// Close the DB so the first Flush fails mid-batch; the rows must be
	// restored into the dirty set instead of being dropped.
	if err := d.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := store.Flush(ctx); err == nil {
		t.Fatal("want flush error on closed db")
	}

	d2, err := db.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = d2.Close() })
	if err := NewQuotaStore(q, d2.Queries, clock).Flush(ctx); err != nil {
		t.Fatalf("retry flush: %v", err)
	}
	rows, err := d2.Queries.ListUsageCountersForDay(ctx, dayDate("2026-08-31"))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].Scope != "app:x" || rows[0].Count != 2 {
		t.Fatalf("want app:x=2 after retry, got %+v", rows)
	}

	// Later touches merge monotonically with the restored value.
	q.Allow("app:x", 100)
	if err := NewQuotaStore(q, d2.Queries, clock).Flush(ctx); err != nil {
		t.Fatalf("flush after touch: %v", err)
	}
	if rows, err = d2.Queries.ListUsageCountersForDay(ctx, dayDate("2026-08-31")); err != nil {
		t.Fatalf("list: %v", err)
	} else if len(rows) != 1 || rows[0].Count != 3 {
		t.Fatalf("want app:x=3 after touch, got %+v", rows)
	}
}
