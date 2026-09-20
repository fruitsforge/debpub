package lock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"path"
	"sync"
	"time"

	"debpub/internal/config"
	"debpub/internal/storage"
)

var (
	// ErrLockTimeout is returned when acquiring the lock exceeds the specified timeout.
	ErrLockTimeout = errors.New("lock: timeout waiting to acquire repository lock")
)

// Locker manages distributed lock acquisition, heartbeat renewal, and release.
type Locker struct {
	backend     storage.StorageBackend
	lockPath    string
	holder      string
	ttl         time.Duration
	timeout     time.Duration
	reason      string
	metadata    map[string]string
	heartbeatCh chan struct{}
	mu          sync.Mutex
}

// LockerOptions configures Locker.
type LockerOptions struct {
	Backend  storage.StorageBackend
	Codename string
	Holder   string
	TTL      time.Duration
	Timeout  time.Duration
	Reason   string
	Metadata map[string]string
}

// NewLocker initializes a new Locker for a repository codename.
func NewLocker(opts LockerOptions) *Locker {
	holder := opts.Holder
	if holder == "" {
		host, _ := os.Hostname()
		holder = fmt.Sprintf("%s-%d", host, os.Getpid())
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = config.DefaultLockTTL
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = config.DefaultLockTimeout
	}

	lockPath := path.Join("dists", opts.Codename, ".lock")

	return &Locker{
		backend:     opts.Backend,
		lockPath:    lockPath,
		holder:      holder,
		ttl:         ttl,
		timeout:     timeout,
		reason:      opts.Reason,
		metadata:    opts.Metadata,
		heartbeatCh: make(chan struct{}),
	}
}

// Acquire attempts to acquire the distributed lock, retrying with jittered exponential backoff until timeout.
func (l *Locker) Acquire(ctx context.Context) error {
	deadline := time.Now().Add(l.timeout)
	attempt := 0

	slog.Info("Attempting to acquire repository lock", "lockPath", l.lockPath, "holder", l.holder, "timeout", l.timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("%w after %v for lock %s", ErrLockTimeout, l.timeout, l.lockPath)
		}

		now := time.Now().UTC()
		info := LockInfo{
			Holder:    l.holder,
			PID:       os.Getpid(),
			CreatedAt: now,
			ExpiresAt: now.Add(l.ttl),
			Reason:    l.reason,
			Metadata:  l.metadata,
		}
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return fmt.Errorf("lock: marshal lock info: %w", err)
		}

		err = l.backend.PutIfNotExist(ctx, l.lockPath, data)
		if err == nil {
			slog.Info("Repository lock acquired successfully", "lockPath", l.lockPath, "expiresAt", info.ExpiresAt)
			l.mu.Lock()
			l.heartbeatCh = make(chan struct{})
			ch := l.heartbeatCh
			l.mu.Unlock()
			l.startHeartbeat(ctx, ch)
			return nil
		}

		if errors.Is(err, storage.ErrAlreadyExists) {
			// Inspect existing lock for staleness
			stale, existingInfo := l.checkStale(ctx)
			if stale {
				// Apply randomized jitter delay to desynchronize concurrent workers seeing the stale lock
				jitter := time.Duration(100+rand.IntN(400)) * time.Millisecond
				select {
				case <-time.After(jitter):
				case <-ctx.Done():
					return ctx.Err()
				}

				// Re-verify that the lock is STILL stale and has not been replaced or refreshed by another worker
				recheckStale, recheckInfo := l.checkStale(ctx)
				if recheckStale && (recheckInfo.Holder == existingInfo.Holder || existingInfo.Holder == "corrupted") {
					slog.Warn("Breaking verified stale/expired repository lock",
						"lockPath", l.lockPath,
						"oldHolder", existingInfo.Holder,
						"expiredAt", existingInfo.ExpiresAt)
					_ = l.backend.Delete(ctx, l.lockPath)
				}
				continue
			}

			attempt++
			backoff := time.Duration(1000+(rand.IntN(1000))) * time.Millisecond
			if attempt > 5 {
				backoff = time.Duration(3000+(rand.IntN(2000))) * time.Millisecond
			}

			slog.Info("Repository lock currently held by another worker, waiting...", "holder", existingInfo.Holder, "expiresAt", existingInfo.ExpiresAt, "retryIn", backoff)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}

		return fmt.Errorf("lock: unexpected storage error: %w", err)
	}
}

func (l *Locker) checkStale(ctx context.Context) (bool, LockInfo) {
	rc, err := l.backend.Get(ctx, l.lockPath)
	if err != nil {
		return false, LockInfo{}
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		return false, LockInfo{}
	}

	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		// Corrupted lock file, consider stale
		return true, LockInfo{Holder: "corrupted"}
	}

	if !info.ExpiresAt.IsZero() && time.Now().UTC().After(info.ExpiresAt) {
		return true, info
	}

	return false, info
}

func (l *Locker) startHeartbeat(ctx context.Context, heartbeatCh <-chan struct{}) {
	interval := max(l.ttl/2, 10*time.Second)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-heartbeatCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now().UTC()
				info := LockInfo{
					Holder:    l.holder,
					PID:       os.Getpid(),
					CreatedAt: now,
					ExpiresAt: now.Add(l.ttl),
					Reason:    l.reason,
					Metadata:  l.metadata,
				}
				data, _ := json.MarshalIndent(info, "", "  ")
				_ = l.backend.PutBytes(context.Background(), l.lockPath, data, "application/json")
				slog.Debug("Refreshed lock TTL heartbeat", "lockPath", l.lockPath, "newExpiresAt", info.ExpiresAt)
			}
		}
	}()
}

// Release releases the lock and terminates the heartbeat routine.
func (l *Locker) Release(ctx context.Context) error {
	l.mu.Lock()
	if l.heartbeatCh != nil {
		close(l.heartbeatCh)
		l.heartbeatCh = nil
	}
	l.mu.Unlock()

	slog.Info("Releasing repository lock", "lockPath", l.lockPath)

	// Verify we still hold the lock before deleting it, preventing clobbering another worker's lock
	rc, err := l.backend.Get(ctx, l.lockPath)
	if err == nil {
		defer func() { _ = rc.Close() }()
		if data, errRead := io.ReadAll(rc); errRead == nil {
			var currentInfo LockInfo
			if errUnmarshal := json.Unmarshal(data, &currentInfo); errUnmarshal == nil {
				if currentInfo.Holder != l.holder {
					slog.Warn("Lock expired and was acquired by another worker before release; skipping delete",
						"lockPath", l.lockPath, "currentHolder", currentInfo.Holder, "ourHolder", l.holder)
					return nil
				}
			}
		}
	} else if errors.Is(err, storage.ErrNotFound) {
		return nil
	}

	return l.backend.Delete(ctx, l.lockPath)
}
