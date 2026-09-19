package repo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blakesmith/ar"

	"debpub/internal/config"
	"debpub/internal/storage"
)

// Helper to construct a valid minimal in-memory .deb file
func createTestDeb(t *testing.T, pkgName, version, arch string) []byte {
	t.Helper()

	// 1. Create control file
	controlContent := []byte(strings.Join([]string{
		"Package: " + pkgName,
		"Version: " + version,
		"Architecture: " + arch,
		"Maintainer: Debpub Test <test@example.com>",
		"Description: Sample test package for debpub",
		"",
	}, "\n"))

	// 2. Wrap control in control.tar.gz
	var controlTarGz bytes.Buffer
	gzWriter := gzip.NewWriter(&controlTarGz)
	tarWriter := tar.NewWriter(gzWriter)

	tarHeader := &tar.Header{
		Name: "./control",
		Mode: 0644,
		Size: int64(len(controlContent)),
	}
	if err := tarWriter.WriteHeader(tarHeader); err != nil {
		t.Fatalf("failed writing tar header: %v", err)
	}
	if _, err := tarWriter.Write(controlContent); err != nil {
		t.Fatalf("failed writing control content to tar: %v", err)
	}
	tarWriter.Close()
	gzWriter.Close()

	// 3. Assemble ar archive with debian-binary and control.tar.gz
	var debBuf bytes.Buffer
	arWriter := ar.NewWriter(&debBuf)
	if err := arWriter.WriteGlobalHeader(); err != nil {
		t.Fatalf("WriteGlobalHeader failed: %v", err)
	}

	// debian-binary entry
	debBin := []byte("2.0\n")
	arWriter.WriteHeader(&ar.Header{
		Name: "debian-binary",
		Mode: 0644,
		Size: int64(len(debBin)),
	})
	arWriter.Write(debBin)

	// control.tar.gz entry
	arWriter.WriteHeader(&ar.Header{
		Name: "control.tar.gz",
		Mode: 0644,
		Size: int64(controlTarGz.Len()),
	})
	arWriter.Write(controlTarGz.Bytes())

	return debBuf.Bytes()
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
	rc.Close()

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
