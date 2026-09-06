package ratelimit

import (
	"context"
	"time"

	"github.com/MunifTanjim/pushport/internal/db"
)

// QuotaStore is the write-behind bridge between an in-memory Quota and the
// usage_counter table. Memory stays authoritative; the DB is a durable backup.
type QuotaStore struct {
	quota *Quota
	q     *db.Queries
	now   func() time.Time
}

func NewQuotaStore(quota *Quota, q *db.Queries, now func() time.Time) *QuotaStore {
	if now == nil {
		now = time.Now
	}
	return &QuotaStore{quota: quota, q: q, now: now}
}

func (s *QuotaStore) today() string { return s.now().UTC().Format("2006-01-02") }

// dayDate converts an internal "2006-01-02" day string to the DATE column type.
// The input always comes from time.Format, so the parse cannot fail.
func dayDate(day string) db.Date {
	d, _ := db.ParseDate(day)
	return d
}

func (s *QuotaStore) Load(ctx context.Context) error {
	day := s.today()
	rows, err := s.q.ListUsageCountersForDay(ctx, dayDate(day))
	if err != nil {
		return err
	}
	counts := make(map[string]int, len(rows))
	for _, r := range rows {
		counts[r.Scope] = int(r.Count)
	}
	s.quota.Load(day, counts)
	return nil
}

// Flush upserts every count changed since the last Flush. On the first upsert
// error it restores every row of the batch to the Quota's dirty set and returns,
// so the next Flush retries the whole batch (values are absolute, and the
// upsert is idempotent).
func (s *QuotaStore) Flush(ctx context.Context) error {
	rows := s.quota.Snapshot()
	for _, row := range rows {
		if err := s.q.UpsertUsageCounter(ctx, db.UpsertUsageCounterParams{
			Scope: row.Tenant, Date: dayDate(row.Day), Count: int64(row.Count),
		}); err != nil {
			s.quota.Restore(rows)
			return err
		}
	}
	return nil
}

func (s *QuotaStore) Cleanup(ctx context.Context) error {
	return s.q.DeleteUsageCountersBefore(ctx, dayDate(s.today()))
}
