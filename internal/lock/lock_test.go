package lock

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"debpub/internal/storage"
)

func TestLockInfoUnmarshalAliases(t *testing.T) {
	// Test backwards compatibility with deb-s3 JSON fields (locked_by, timestamp)
	legacyJSON := `{
		"locked_by": "azure-runner-42",
		"timestamp": "2026-09-16T12:00:00Z",
		"expiration": "2026-09-16T12:10:00Z"
	}`

	var info LockInfo
	if err := json.Unmarshal([]byte(legacyJSON), &info); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if info.Holder != "azure-runner-42" {
		t.Errorf("Holder alias not resolved: got %s", info.Holder)
	}
	if info.CreatedAt.Year() != 2026 {
		t.Errorf("CreatedAt timestamp mismatch: %v", info.CreatedAt)
	}
	if info.ExpiresAt.Minute() != 10 {
		t.Errorf("ExpiresAt mismatch: %v", info.ExpiresAt)
	}
}

func TestLockerAcquireAndRelease(t *testing.T) {
	tempDir := t.TempDir()
	backend, _ := storage.NewFileBackend(tempDir)

	ctx := context.Background()
	locker := NewLocker(LockerOptions{
		Backend:  backend,
		Codename: "stable",
		Holder:   "worker-1",
		Timeout:  2 * time.Second,
		TTL:      1 * time.Minute,
	})

	if err := locker.Acquire(ctx); err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	// Verify lock file exists
	exists, _ := backend.Exists(ctx, "dists/stable/.lock")
	if !exists {
		t.Fatalf("lock file does not exist on disk")
	}

	// Second locker should fail on timeout
	locker2 := NewLocker(LockerOptions{
		Backend:  backend,
		Codename: "stable",
		Holder:   "worker-2",
		Timeout:  500 * time.Millisecond,
		TTL:      1 * time.Minute,
	})

	err := locker2.Acquire(ctx)
	if err == nil {
		t.Fatalf("expected locker2 to fail with timeout, but it succeeded")
	}

	// Release first lock
	if err := locker.Release(ctx); err != nil {
		t.Fatalf("release failed: %v", err)
	}

	// Now locker2 should succeed
	ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := locker2.Acquire(ctx2); err != nil {
		t.Fatalf("locker2 acquire after release failed: %v", err)
	}
	_ = locker2.Release(ctx)
}

func TestLockerStaleRecovery(t *testing.T) {
	tempDir := t.TempDir()
	backend, _ := storage.NewFileBackend(tempDir)
	ctx := context.Background()

	// Place an already-expired lock
	past := time.Now().UTC().Add(-10 * time.Minute)
	expiredInfo := LockInfo{
		Holder:    "crashed-runner",
		CreatedAt: past.Add(-5 * time.Minute),
		ExpiresAt: past,
	}
	data, _ := json.Marshal(expiredInfo)
	_ = backend.PutBytes(ctx, "dists/stable/.lock", data, "application/json")

	// New locker should automatically detect staleness, break it, and acquire
	locker := NewLocker(LockerOptions{
		Backend:  backend,
		Codename: "stable",
		Holder:   "active-runner",
		Timeout:  2 * time.Second,
	})

	if err := locker.Acquire(ctx); err != nil {
		t.Fatalf("failed to break stale lock and acquire: %v", err)
	}

	_ = locker.Release(ctx)
}

func TestLockerConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	backend, _ := storage.NewFileBackend(tempDir)

	var wg sync.WaitGroup
	workers := 5
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			locker := NewLocker(LockerOptions{
				Backend:  backend,
				Codename: "stable",
				Holder:   "concurrent-worker",
				Timeout:  10 * time.Second,
			})

			if err := locker.Acquire(ctx); err == nil {
				// simulate critical section
				time.Sleep(50 * time.Millisecond)
				_ = locker.Release(ctx)

				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	if successCount != workers {
		t.Fatalf("expected all %d workers to eventually acquire and release, got %d", workers, successCount)
	}
}
