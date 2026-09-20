package repo

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"debpub/internal/config"
	"debpub/internal/storage"
	"debpub/internal/testutil"
)

// Helper to construct a valid minimal in-memory .deb file
func createTestDeb(t *testing.T, pkgName, version, arch string) []byte {
	t.Helper()
	return testutil.CreateTestDeb(t, testutil.DebOptions{
		Package:      pkgName,
		Version:      version,
		Architecture: arch,
		Description:  "Sample test package for debpub",
	})
}

func TestPublisherEndToEnd(t *testing.T) {
	repoDir := t.TempDir()
	backend, err := storage.NewFileBackend(repoDir)
	if err != nil {
		t.Fatalf("failed to create FileBackend: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Codename = "stable"
	cfg.Component = "main"
	cfg.Origin = "DebpubTest"
	cfg.Label = "DebpubTest"
	cfg.PreserveVersions = true

	pub := NewPublisher(cfg, backend, nil)

	// Create 2 test deb packages
	pkg1Data := createTestDeb(t, "sample-service", "1.0.0", "amd64")
	pkg2Data := createTestDeb(t, "sample-service", "1.1.0", "amd64")

	debFile1 := filepath.Join(repoDir, "sample-service_1.0.0_amd64.deb")
	debFile2 := filepath.Join(repoDir, "sample-service_1.1.0_amd64.deb")

	if err := os.WriteFile(debFile1, pkg1Data, 0644); err != nil {
		t.Fatalf("write deb1 failed: %v", err)
	}
	if err := os.WriteFile(debFile2, pkg2Data, 0644); err != nil {
		t.Fatalf("write deb2 failed: %v", err)
	}

	ctx := context.Background()

	// 1. Publish first package
	err = pub.PublishDebFiles(ctx, []string{debFile1})
	if err != nil {
		t.Fatalf("PublishDebFiles pkg1 failed: %v", err)
	}

	// Verify Phase 1: pool .deb file exists
	poolPath1 := "pool/main/s/sample-service/sample-service_1.0.0_amd64.deb"
	exists, _ := backend.Exists(ctx, poolPath1)
	if !exists {
		t.Fatalf("expected %s in pool, but does not exist", poolPath1)
	}

	// Verify Phase 2: Index variants exist (Packages, Packages.gz, Packages.bz2, Packages.xz)
	baseIdx := "dists/stable/main/binary-amd64/Packages"
	for _, ext := range []string{"", ".gz", ".bz2", ".xz"} {
		exists, _ := backend.Exists(ctx, baseIdx+ext)
		if !exists {
			t.Errorf("expected index %s, but missing", baseIdx+ext)
		}
	}

	// Verify Phase 3: Release manifest exists
	exists, _ = backend.Exists(ctx, "dists/stable/Release")
	if !exists {
		t.Fatalf("expected dists/stable/Release, but missing")
	}

	// 2. Publish second package with version preservation
	err = pub.PublishDebFiles(ctx, []string{debFile2})
	if err != nil {
		t.Fatalf("PublishDebFiles pkg2 failed: %v", err)
	}

	poolPath2 := "pool/main/s/sample-service/sample-service_1.1.0_amd64.deb"
	exists, _ = backend.Exists(ctx, poolPath2)
	if !exists {
		t.Fatalf("expected %s in pool, but does not exist", poolPath2)
	}

	// Read Packages index and verify both 1.0.0 and 1.1.0 are present
	rc, err := backend.Get(ctx, baseIdx)
	if err != nil {
		t.Fatalf("failed reading Packages index: %v", err)
	}
	idxContent, _ := io.ReadAll(rc)
	if err := rc.Close(); err != nil {
		t.Fatalf("failed to close Packages reader: %v", err)
	}

	strContent := string(idxContent)
	if !strings.Contains(strContent, "Version: 1.0.0") || !strings.Contains(strContent, "Version: 1.1.0") {
		t.Errorf("Packages index does not contain both versions:\n%s", strContent)
	}

	// Verify lock was released cleanly
	lockExists, _ := backend.Exists(ctx, "dists/stable/.lock")
	if lockExists {
		t.Errorf(".lock file was not released upon completion")
	}
}

func TestPublisherBatchWithVersionSorting(t *testing.T) {
	repoDir := t.TempDir()
	backend, err := storage.NewFileBackend(repoDir)
	if err != nil {
		t.Fatalf("failed to create FileBackend: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Codename = "stable"
	cfg.Component = "main"
	cfg.PreserveVersions = false // Test that latest version wins when preserve-versions=false

	pub := NewPublisher(cfg, backend, nil)

	// Note: in ASCII alphabetical order, "1.10.0" comes BEFORE "1.2.0".
	// If candidates were sorted purely by filename string, 1.2.0 would be processed after 1.10.0,
	// replacing the newer version with the older version.
	// With Debian version sorting, 1.2.0 is merged first and 1.10.0 is merged last.
	pkgOlder := createTestDeb(t, "version-test", "1.2.0", "amd64")
	pkgNewer := createTestDeb(t, "version-test", "1.10.0", "amd64")

	debNewer := filepath.Join(repoDir, "version-test_1.10.0_amd64.deb")
	debOlder := filepath.Join(repoDir, "version-test_1.2.0_amd64.deb")

	if err := os.WriteFile(debNewer, pkgNewer, 0644); err != nil {
		t.Fatalf("write newer deb failed: %v", err)
	}
	if err := os.WriteFile(debOlder, pkgOlder, 0644); err != nil {
		t.Fatalf("write older deb failed: %v", err)
	}

	ctx := context.Background()
	// Pass in arbitrary order
	if err := pub.PublishDebFiles(ctx, []string{debNewer, debOlder}); err != nil {
		t.Fatalf("PublishDebFiles batch failed: %v", err)
	}

	// Verify that the index retained 1.10.0 and not 1.2.0
	baseIdx := "dists/stable/main/binary-amd64/Packages"
	rc, err := backend.Get(ctx, baseIdx)
	if err != nil {
		t.Fatalf("failed reading Packages index: %v", err)
	}
	idxContent, _ := io.ReadAll(rc)
	if err := rc.Close(); err != nil {
		t.Fatalf("failed to close Packages reader: %v", err)
	}

	strContent := string(idxContent)
	if !strings.Contains(strContent, "Version: 1.10.0") {
		t.Errorf("expected latest version 1.10.0 in Packages index, got:\n%s", strContent)
	}
	if strings.Contains(strContent, "Version: 1.2.0") {
		t.Errorf("older version 1.2.0 should have been replaced when PreserveVersions=false, got:\n%s", strContent)
	}
}
