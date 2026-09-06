package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterReserveCancelRestores(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewLimiter(func() time.Time { return now })

	cancel, ok, _ := l.Reserve("k", 60, 1)
	if !ok || cancel == nil {
		t.Fatalf("reserve should succeed, ok=%v cancelNil=%v", ok, cancel == nil)
	}
	if l.Allowed("k", 60, 1) {
		t.Fatal("token should be consumed after reserve")
	}

	cancel()
	if !l.Allowed("k", 60, 1) {
		t.Fatal("cancel should restore the token")
	}

	if _, ok, _ := l.Reserve("k", 60, 1); !ok {
		t.Fatal("reserve of the restored token should succeed")
	}
	if _, ok, retry := l.Reserve("k", 60, 1); ok || retry <= 0 {
		t.Fatalf("empty-bucket reserve should deny with a retry, ok=%v retry=%v", ok, retry)
	}

	c, ok, _ := l.Reserve("x", 0, 0)
	if !ok || c == nil {
		t.Fatalf("unlimited reserve should allow, ok=%v cancelNil=%v", ok, c == nil)
	}
	c() // must not panic
}

func TestLimiterBurstThenDeny(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewLimiter(func() time.Time { return now })
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("k", 60, 3); !ok {
			t.Fatalf("token %d should be allowed", i)
		}
	}
	ok, retry := l.Allow("k", 60, 3)
	if ok {
		t.Fatal("4th request should be denied")
	}
	if retry <= 0 || retry > time.Second {
		t.Fatalf("retryAfter = %v, want (0,1s]", retry)
	}
}

func TestLimiterRefill(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewLimiter(func() time.Time { return now })
	if ok, _ := l.Allow("k", 60, 1); !ok {
		t.Fatal("first should be allowed")
	}
	if ok, _ := l.Allow("k", 60, 1); ok {
		t.Fatal("second should be denied (bucket empty)")
	}
	now = now.Add(time.Second) // one token refills at 60/min = 1/s
	if ok, _ := l.Allow("k", 60, 1); !ok {
		t.Fatal("should be allowed after refill")
	}
}

func TestLimiterDisabled(t *testing.T) {
	l := NewLimiter(nil)
	for i := 0; i < 100; i++ {
		if ok, _ := l.Allow("k", 0, 0); !ok {
			t.Fatal("perMin<=0 should always allow")
		}
	}
	if l.numKeys() != 0 {
		t.Fatalf("disabled calls should create no buckets, got %d", l.numKeys())
	}
}

func TestLimiterCleanupEvictsIdle(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewLimiter(func() time.Time { return now })
	l.Allow("k", 60, 1)
	now = now.Add(time.Hour)
	l.Cleanup(time.Minute)
	if l.numKeys() != 0 {
		t.Fatalf("idle key should be evicted, got %d keys", l.numKeys())
	}
}

func TestLimiterKeysAreIndependent(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewLimiter(func() time.Time { return now })
	if ok, _ := l.Allow("a", 60, 1); !ok {
		t.Fatal("a first allowed")
	}
	if ok, _ := l.Allow("a", 60, 1); ok {
		t.Fatal("a second denied")
	}
	if ok, _ := l.Allow("b", 60, 1); !ok {
		t.Fatal("b has its own bucket, should be allowed")
	}
}

func TestLimiterAllowedPeekDoesNotConsume(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewLimiter(func() time.Time { return now })

	if !l.Allowed("k", 60, 1) {
		t.Fatal("fresh key should peek allowed")
	}
	if l.numKeys() != 0 {
		t.Fatalf("peek should create no bucket, got %d keys", l.numKeys())
	}

	if !l.Allowed("k", 0, 0) {
		t.Fatal("disabled limiter should peek allowed")
	}

	if ok, _ := l.Allow("k", 60, 1); !ok {
		t.Fatal("first consume should be allowed")
	}
	if l.Allowed("k", 60, 1) {
		t.Fatal("drained bucket should peek blocked")
	}
	if l.Allowed("k", 60, 1) {
		t.Fatal("peek must not refill or consume; still blocked")
	}
}

func TestLimiterReconfigShrinksBurst(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewLimiter(func() time.Time { return now })
	for i := 0; i < 5; i++ {
		if ok, _ := l.Allow("k", 600, 5); !ok {
			t.Fatalf("token %d should be allowed at burst 5", i)
		}
	}
	if ok, _ := l.Allow("k", 60, 1); ok {
		t.Fatal("after reconfig to burst 1 on an empty bucket, should deny")
	}
}
