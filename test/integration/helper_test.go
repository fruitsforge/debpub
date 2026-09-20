//go:build integration

package integration

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"debpub/internal/testutil"
)

// Helper to construct a valid minimal in-memory .deb file
func createTestDeb(t *testing.T, pkgName, version, arch, description string) []byte {
	t.Helper()
	return testutil.CreateTestDeb(t, testutil.DebOptions{
		Package:      pkgName,
		Version:      version,
		Architecture: arch,
		Description:  description,
	})
}

// writeDebToDisk writes a test deb file to disk and returns its path
func writeDebToDisk(t *testing.T, dir, pkgName, version, arch string) string {
	t.Helper()
	data := createTestDeb(t, pkgName, version, arch, fmt.Sprintf("Integration test package for %s", pkgName))
	filePath := filepath.Join(dir, fmt.Sprintf("%s_%s_%s.deb", pkgName, version, arch))
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("failed writing test deb: %v", err)
	}
	return filePath
}

func readAllToString(t *testing.T, r io.ReadCloser) string {
	t.Helper()
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed reading stream: %v", err)
	}
	return string(b)
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
