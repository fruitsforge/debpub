package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
)

func TestFileBackendCRUDAndLock(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewFileBackend(tempDir)
	if err != nil {
		t.Fatalf("failed to create FileBackend: %v", err)
	}

	ctx := context.Background()

	// 1. Put and Exists
	testData := []byte("Sample Debian Index Content")
	err = backend.Put(ctx, "dists/stable/main/binary-amd64/Packages", bytes.NewReader(testData), int64(len(testData)), "text/plain")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	exists, err := backend.Exists(ctx, "dists/stable/main/binary-amd64/Packages")
	if err != nil || !exists {
		t.Fatalf("Exists returned false/error: %v", err)
	}

	// 2. Get
	r, err := backend.Get(ctx, "dists/stable/main/binary-amd64/Packages")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer func() { _ = r.Close() }()
	content, errRead := io.ReadAll(r)
	if errRead != nil {
		t.Fatalf("ReadAll failed: %v", errRead)
	}
	if !bytes.Equal(content, testData) {
		t.Fatalf("content mismatch: got %q, want %q", string(content), string(testData))
	}

	// 3. List
	files, err := backend.List(ctx, "dists/stable")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 1 || filepath.ToSlash(files[0]) != "dists/stable/main/binary-amd64/Packages" {
		t.Fatalf("List mismatch: %v", files)
	}

	// 4. PutIfNotExist (Atomic exclusive write)
	lockData := []byte(`{"holder":"test-runner"}`)
	err = backend.PutIfNotExist(ctx, "dists/stable/.lock", lockData)
	if err != nil {
		t.Fatalf("first PutIfNotExist failed: %v", err)
	}

	// Second attempt should return ErrAlreadyExists
	err = backend.PutIfNotExist(ctx, "dists/stable/.lock", lockData)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}

	// 5. Delete
	err = backend.Delete(ctx, "dists/stable/.lock")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	exists, _ = backend.Exists(ctx, "dists/stable/.lock")
	if exists {
		t.Fatalf("deleted lock still exists")
	}
}
