//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"debpub/internal/config"
	"debpub/internal/repo"
	"debpub/internal/storage"
)

func getSFTPBackend(basePath string) (*storage.SFTPBackend, error) {
	host := getEnvOrDefault("SFTP_HOST", "sftp")
	portStr := getEnvOrDefault("SFTP_PORT", "22")
	port, _ := strconv.Atoi(portStr)
	user := getEnvOrDefault("SFTP_USER", "testuser")
	pass := getEnvOrDefault("SFTP_PASSWORD", "testpassword")

	return storage.NewSFTPBackend(storage.SFTPOptions{
		Host:     host,
		Port:     port,
		User:     user,
		Password: pass,
		BasePath: basePath,
	})
}

func TestSFTPBackend_CRUDAndLocking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	basePath := fmt.Sprintf("/home/testuser/debian/test-crud-%d", time.Now().UnixNano())
	backend, err := getSFTPBackend(basePath)
	if err != nil {
		t.Fatalf("getSFTPBackend failed: %v", err)
	}

	t.Run("Put and Exists", func(t *testing.T) {
		data := []byte("sftp test payload")
		err := backend.Put(ctx, "sub/file.txt", bytes.NewReader(data), int64(len(data)), "text/plain")
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}

		exists, err := backend.Exists(ctx, "sub/file.txt")
		if err != nil || !exists {
			t.Fatalf("Exists returned false or error: %v", err)
		}
	})

	t.Run("Get Content", func(t *testing.T) {
		rc, err := backend.Get(ctx, "sub/file.txt")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		got := readAllToString(t, rc)
		if got != "sftp test payload" {
			t.Fatalf("content mismatch: got %q, want 'sftp test payload'", got)
		}
	})

	t.Run("List with Subdirectory", func(t *testing.T) {
		files, err := backend.List(ctx, "sub")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(files) == 0 {
			t.Fatalf("expected files in list, got 0")
		}
	})

	t.Run("Atomic POSIX Lock PutIfNotExist", func(t *testing.T) {
		lockPath := "dists/bookworm/.lock"
		lockPayload := []byte(`{"holder":"sftp-runner-1","created_at":"2026-09-19T20:00:00Z"}`)

		// First acquisition must succeed
		err := backend.PutIfNotExist(ctx, lockPath, lockPayload)
		if err != nil {
			t.Fatalf("initial PutIfNotExist failed: %v", err)
		}

		// Second acquisition must fail with ErrAlreadyExists
		secondPayload := []byte(`{"holder":"sftp-runner-2","created_at":"2026-09-19T20:00:01Z"}`)
		err = backend.PutIfNotExist(ctx, lockPath, secondPayload)
		if !errors.Is(err, storage.ErrAlreadyExists) {
			t.Fatalf("expected ErrAlreadyExists on lock collision, got: %v", err)
		}

		// Cleanup lock
		if err := backend.Delete(ctx, lockPath); err != nil {
			t.Fatalf("Delete lock failed: %v", err)
		}

		exists, _ := backend.Exists(ctx, lockPath)
		if exists {
			t.Fatalf("deleted lock still exists")
		}
	})
}

func TestSFTPBackend_PublisherEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	basePath := fmt.Sprintf("/home/testuser/debian/e2e-sftp-%d", time.Now().UnixNano())
	backend, err := getSFTPBackend(basePath)
	if err != nil {
		t.Fatalf("getSFTPBackend failed: %v", err)
	}

	cfg := &config.Config{
		Codename:    "bookworm",
		Component:   "main",
		Sign:        false,
		LockEnabled: true,
	}

	publisher := repo.NewPublisher(cfg, backend, nil)

	tempDir := t.TempDir()
	deb1 := writeDebToDisk(t, tempDir, "app-sftp", "2.0.0", "amd64")

	t.Run("Publish Initial Package over SFTP", func(t *testing.T) {
		err := publisher.PublishDebFiles(ctx, []string{deb1})
		if err != nil {
			t.Fatalf("PublishDebFiles failed over SFTP: %v", err)
		}

		// Verify pool file exists
		poolPath := "pool/main/a/app-sftp/app-sftp_2.0.0_amd64.deb"
		exists, err := backend.Exists(ctx, poolPath)
		if err != nil || !exists {
			t.Fatalf("expected pool file %q to exist on SFTP", poolPath)
		}

		// Verify Packages index contains app-sftp
		pkgPath := "dists/bookworm/main/binary-amd64/Packages"
		rc, err := backend.Get(ctx, pkgPath)
		if err != nil {
			t.Fatalf("Get Packages failed: %v", err)
		}
		packagesContent := readAllToString(t, rc)
		if !bytes.Contains([]byte(packagesContent), []byte("Package: app-sftp")) {
			t.Fatalf("Packages does not contain 'Package: app-sftp':\n%s", packagesContent)
		}

		// Verify Packages.bz2 and other formats exist
		for _, compExt := range []string{".gz", ".bz2", ".xz"} {
			cExists, err := backend.Exists(ctx, pkgPath+compExt)
			if err != nil || !cExists {
				t.Fatalf("expected compressed index %s to exist on SFTP", pkgPath+compExt)
			}
		}

		// Verify Release manifest exists
		releasePath := "dists/bookworm/Release"
		rExists, err := backend.Exists(ctx, releasePath)
		if err != nil || !rExists {
			t.Fatalf("expected Release manifest to exist on SFTP")
		}

		// Verify lock was released
		lockPath := "dists/bookworm/.lock"
		lExists, _ := backend.Exists(ctx, lockPath)
		if lExists {
			t.Fatalf("expected .lock to be removed after publish on SFTP")
		}
	})

	t.Run("Publish Package Upgrade over SFTP", func(t *testing.T) {
		deb2 := writeDebToDisk(t, tempDir, "app-sftp", "2.1.0", "amd64")
		err := publisher.PublishDebFiles(ctx, []string{deb2})
		if err != nil {
			t.Fatalf("PublishDebFiles upgrade failed over SFTP: %v", err)
		}

		// Verify updated Packages contains version 2.1.0
		pkgPath := "dists/bookworm/main/binary-amd64/Packages"
		rc, err := backend.Get(ctx, pkgPath)
		if err != nil {
			t.Fatalf("Get Packages failed: %v", err)
		}
		packagesContent := readAllToString(t, rc)
		if !bytes.Contains([]byte(packagesContent), []byte("Version: 2.1.0")) {
			t.Fatalf("Packages does not contain updated 'Version: 2.1.0':\n%s", packagesContent)
		}
	})
}
