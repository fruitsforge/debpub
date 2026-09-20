//go:build integration

package integration

import (
	"io"
	"os"
	"testing"

	"debpub/internal/testutil"
)

// writeDebToDisk writes a test deb file to disk and returns its path
func writeDebToDisk(t *testing.T, dir, pkgName, version, arch string) string {
	t.Helper()
	return testutil.WriteTestDebToDisk(t, dir, pkgName, version, arch)
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
