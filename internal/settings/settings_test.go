package settings_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/settings"
)

type stubQ struct {
	row  db.ServerSetting
	err  error
	hits int
}

func (s *stubQ) GetServerSetting(ctx context.Context) (db.ServerSetting, error) {
	s.hits++
	return s.row, s.err
}

func TestReaderGetCachesAndFlush(t *testing.T) {
	q := &stubQ{row: db.ServerSetting{RetryMaxAttempts: 7, RetryBaseBackoff: "100ms", RetryMaxBackoff: "900ms", AuthFailIpPerMin: 2, AuthFailIpBurst: 4}}
	r := settings.NewReader(q)

	got := r.Get(context.Background())
	if got.RetryMaxAttempts != 7 || got.AuthFailIPPerMin != 2 {
		t.Fatalf("bad get: %+v", got)
	}
	_ = r.Get(context.Background())
	if q.hits != 1 {
		t.Fatalf("expected cached read, got %d hits", q.hits)
	}
	r.Flush()
	_ = r.Get(context.Background())
	if q.hits != 2 {
		t.Fatalf("expected re-read after flush, got %d hits", q.hits)
	}
}

func TestReaderFailsOpenToDefaults(t *testing.T) {
	q := &stubQ{err: errors.New("db down")}
	r := settings.NewReader(q)
	got := r.Get(context.Background())
	if got != settings.Defaults {
		t.Fatalf("want defaults on error, got %+v", got)
	}
	_ = r.Get(context.Background())
	if q.hits != 2 {
		t.Fatalf("error result must not cache, hits=%d", q.hits)
	}
}

func TestReaderRetryConfigMapping(t *testing.T) {
	q := &stubQ{row: db.ServerSetting{RetryMaxAttempts: 4, RetryBaseBackoff: "250ms", RetryMaxBackoff: "3s", AuthFailIpPerMin: 6, AuthFailIpBurst: 8}}
	r := settings.NewReader(q)
	a, base, max := r.RetryConfig(context.Background())
	if a != 4 || base != 250*time.Millisecond || max != 3000*time.Millisecond {
		t.Fatalf("retry mapping wrong: %d %v %v", a, base, max)
	}
	pm, b := r.AuthFailConfig(context.Background())
	if pm != 6 || b != 8 {
		t.Fatalf("authfail mapping wrong: %d %d", pm, b)
	}
}
