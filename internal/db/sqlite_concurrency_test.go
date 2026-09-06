package db

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MunifTanjim/pushport/internal/db/sqlc"
)

func TestFileBackedConcurrentReads(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "relay.db")
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.CreateApp(ctx, sqlc.CreateAppParams{ID: "t1", Name: "app", CreatedAt: 1}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := d.GetApp(ctx, "t1"); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent read: %v", err)
	}
}
