package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrAlreadyExists is returned when PutIfNotExist fails because the file is present.
	ErrAlreadyExists = errors.New("storage: object already exists")
	// ErrNotFound is returned when an object does not exist.
	ErrNotFound = errors.New("storage: object not found")
)

// FileBackend implements StorageBackend for the local filesystem or mounted volumes.
type FileBackend struct {
	BaseDir string
}

// NewFileBackend creates a new FileBackend pointing to baseDir.
func NewFileBackend(baseDir string) (*FileBackend, error) {
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("file storage: invalid base directory: %w", err)
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, fmt.Errorf("file storage: failed creating base directory: %w", err)
	}
	return &FileBackend{BaseDir: abs}, nil
}

func (f *FileBackend) resolve(path string) string {
	clean := filepath.Clean(strings.TrimPrefix(path, "/"))
	return filepath.Join(f.BaseDir, clean)
}

// Get retrieves an object reader from local filesystem storage.
func (f *FileBackend) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := f.resolve(path)
	file, err := os.Open(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("file storage get: %w", err)
	}
	return file, nil
}

// Put writes an object to local filesystem storage atomically using a temporary file.
func (f *FileBackend) Put(ctx context.Context, path string, data io.Reader, size int64, contentType string) error {
	fullPath := f.resolve(path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("file storage put mkdir: %w", err)
	}

	// Write to temp file then atomic rename
	tmpFile, err := os.CreateTemp(filepath.Dir(fullPath), ".debpub-tmp-*")
	if err != nil {
		return fmt.Errorf("file storage create temp: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := io.Copy(tmpFile, data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("file storage copy: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("file storage close temp: %w", err)
	}

	if err := os.Rename(tmpName, fullPath); err != nil {
		return fmt.Errorf("file storage rename: %w", err)
	}
	return nil
}

// PutBytes is a convenience helper storing a raw byte slice.
func (f *FileBackend) PutBytes(ctx context.Context, path string, data []byte, contentType string) error {
	return f.Put(ctx, path, bytes.NewReader(data), int64(len(data)), contentType)
}

// Delete removes an object from storage.
func (f *FileBackend) Delete(ctx context.Context, path string) error {
	fullPath := f.resolve(path)
	err := os.Remove(fullPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("file storage delete: %w", err)
	}
	return nil
}

// Exists checks if an object exists.
func (f *FileBackend) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := f.resolve(path)
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// List returns relative object paths matching a prefix.
func (f *FileBackend) List(ctx context.Context, prefix string) ([]string, error) {
	searchDir := f.resolve(prefix)
	var matches []string

	// If searchDir doesn't exist, return empty
	if _, err := os.Stat(searchDir); errors.Is(err, os.ErrNotExist) {
		return matches, nil
	}

	err := filepath.Walk(searchDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, err := filepath.Rel(f.BaseDir, p)
			if err != nil {
				return err
			}
			matches = append(matches, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("file storage list: %w", err)
	}
	return matches, nil
}

// PutIfNotExist stores data atomically if and only if the object does not already exist.
func (f *FileBackend) PutIfNotExist(ctx context.Context, path string, data []byte) error {
	fullPath := f.resolve(path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("file storage mkdir: %w", err)
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	file, err := os.OpenFile(fullPath, flags, 0644)
	if err != nil {
		if os.IsExist(err) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("file storage open exclusive: %w", err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("file storage write: %w", err)
	}
	return nil
}
