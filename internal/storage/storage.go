package storage

import (
	"context"
	"io"
)

// StorageBackend defines the unified object storage operations for Debian repositories.
type StorageBackend interface {
	// Get retrieves an object reader. Returns os.ErrNotExist or custom NotFound if not found.
	Get(ctx context.Context, path string) (io.ReadCloser, error)

	// Put stores an object with specified size and content type.
	Put(ctx context.Context, path string, data io.Reader, size int64, contentType string) error

	// PutBytes is a convenience helper storing a raw byte slice.
	PutBytes(ctx context.Context, path string, data []byte, contentType string) error

	// Delete removes an object from storage.
	Delete(ctx context.Context, path string) error

	// Exists checks if an object exists.
	Exists(ctx context.Context, path string) (bool, error)

	// List returns relative object paths matching a prefix.
	List(ctx context.Context, prefix string) ([]string, error)

	// PutIfNotExist stores data atomically if and only if the object does not already exist.
	// Returns ErrAlreadyExists if the object already exists.
	PutIfNotExist(ctx context.Context, path string, data []byte) error
}
