package ratelimit

import (
	"sync"
	"time"
)

// Quota tracks per-tenant daily push counts in memory, the authoritative source
// for enforcement; Snapshot/Load persist them to the DB across restarts. Counts
// reset at the UTC day boundary.
type Quota struct {
	now func() time.Time

	mu      sync.Mutex
	day     string
	counts  map[string]int
	dirty   map[string]struct{}       // tenants changed since the last Snapshot
	pending map[string]map[string]int // outgoing-day counts held over a rollover: day -> tenant -> count
}

func NewQuota(now func() time.Time) *Quota {
	if now == nil {
		now = time.Now
	}
	return &Quota{
		now:     now,
		counts:  map[string]int{},
		dirty:   map[string]struct{}{},
		pending: map[string]map[string]int{},
	}
}

type quotaCheck struct {
	tenant string
	limit  int
}

// Admit peeks every check before incrementing any, so a request denied by one
// quota never consumes another's budget. Returns (false, retryAfter, tenant) on
// the first over-limit check, retryAfter measured to UTC midnight.
func (q *Quota) Admit(checks ...quotaCheck) (bool, time.Duration, string) {
	now := q.now().UTC()
	day := now.Format("2006-01-02")
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rollLocked(day)

	for _, c := range checks {
		if c.limit <= 0 {
			continue
		}
		if q.counts[c.tenant] >= c.limit {
			next := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
			return false, next.Sub(now), c.tenant
		}
	}
	for _, c := range checks {
		if c.limit <= 0 {
			continue
		}
		q.counts[c.tenant]++
		q.dirty[c.tenant] = struct{}{}
	}
	return true, 0, ""
}

func (q *Quota) Allow(tenant string, limit int) (bool, time.Duration) {
	ok, retry, _ := q.Admit(quotaCheck{tenant: tenant, limit: limit})
	return ok, retry
}

// rollLocked resets counts when the UTC day changes, preserving the outgoing
// day's unflushed counts for the next Snapshot. Caller holds q.mu.
func (q *Quota) rollLocked(day string) {
	if day == q.day {
		return
	}
	if q.day != "" && len(q.dirty) > 0 {
		old := q.pending[q.day]
		if old == nil {
			old = map[string]int{}
			q.pending[q.day] = old
		}
		for t := range q.dirty {
			old[t] = q.counts[t]
		}
	}
	q.day = day
	q.counts = map[string]int{}
	q.dirty = map[string]struct{}{}
}

// CounterRow is a single (tenant, day) counter value for persistence.
type CounterRow struct {
	Tenant string
	Day    string
	Count  int
}

// Snapshot returns and clears every counter changed since the last Snapshot,
// including outgoing-day counts held over a rollover. Values are absolute, so
// the upsert is idempotent.
func (q *Quota) Snapshot() []CounterRow {
	q.mu.Lock()
	defer q.mu.Unlock()
	var rows []CounterRow
	for day, m := range q.pending {
		for t, c := range m {
			rows = append(rows, CounterRow{Tenant: t, Day: day, Count: c})
		}
	}
	q.pending = map[string]map[string]int{}
	for t := range q.dirty {
		rows = append(rows, CounterRow{Tenant: t, Day: q.day, Count: q.counts[t]})
	}
	q.dirty = map[string]struct{}{}
	return rows
}

// Load seeds the in-memory counts for day from persisted rows (startup
// recovery). The loaded counts are not marked dirty — the DB already holds them.
func (q *Quota) Load(day string, counts map[string]int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.day = day
	q.counts = make(map[string]int, len(counts))
	for t, c := range counts {
		q.counts[t] = c
	}
	q.dirty = map[string]struct{}{}
}

// Restore re-marks rows as changed after a failed persistence attempt, so a
// later Snapshot returns them again. Rows for the current day update the
// in-memory counts (never decreasing them), rows for other days go back to
// pending.
func (q *Quota) Restore(rows []CounterRow) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, r := range rows {
		if r.Day == q.day {
			if r.Count > q.counts[r.Tenant] {
				q.counts[r.Tenant] = r.Count
			}
			q.dirty[r.Tenant] = struct{}{}
			continue
		}
		m := q.pending[r.Day]
		if m == nil {
			m = map[string]int{}
			q.pending[r.Day] = m
		}
		if v, ok := m[r.Tenant]; !ok || r.Count > v {
			m[r.Tenant] = r.Count
		}
	}
}
