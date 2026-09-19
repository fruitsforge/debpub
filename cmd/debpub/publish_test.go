package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blakesmith/ar"
)

func TestBuildStorageBackendS3Options(t *testing.T) {
	tempDir := t.TempDir()

	// Write dummy AWS config and credentials files
	awsConfigFile := filepath.Join(tempDir, "config")
	awsCredsFile := filepath.Join(tempDir, "credentials")

	configContent := `[profile custom_profile]
region = us-west-2
`
	credsContent := `[custom_profile]
aws_access_key_id = TEST_ACCESS_KEY
aws_secret_access_key = TEST_SECRET_KEY
`
	if err := os.WriteFile(awsConfigFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write dummy AWS config: %v", err)
	}
	if err := os.WriteFile(awsCredsFile, []byte(credsContent), 0600); err != nil {
		t.Fatalf("failed to write dummy AWS credentials: %v", err)
	}

	t.Setenv("AWS_CONFIG_FILE", awsConfigFile)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", awsCredsFile)

	ctx := context.Background()

	// Configure cfg with S3 storage, custom profile, and custom region
	cfg.Storage = "s3"
	cfg.Bucket = "test-bucket"
	cfg.S3Profile = "custom_profile"
	cfg.S3Region = "eu-west-1"

	backend, err := buildStorageBackend(ctx)
	if err != nil {
		t.Fatalf("buildStorageBackend failed with custom S3Profile and S3Region: %v", err)
	}
	if backend == nil {
		t.Fatal("expected non-nil StorageBackend")
	}
}

func TestPublishCmdArgCollection(t *testing.T) {
	tempDir := t.TempDir()

	// Ensure codename is set
	cfg.Codename = "stable"
	cfg.Storage = "file"
	cfg.LocalDir = filepath.Join(tempDir, "repo")

	// Calling publishCmd.RunE with a non-existent path should return package collection error
	err := publishCmd.RunE(publishCmd, []string{filepath.Join(tempDir, "does-not-exist.deb")})
	if err == nil {
		t.Fatal("expected error for non-existent deb file, got nil")
	}
	if !filepath.IsAbs(tempDir) {
		t.Fatalf("expected absolute tempDir")
	}
}

func TestPublishCmdWithGlobSelection(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	incomingDir := filepath.Join(tempDir, "incoming")

	if err := os.MkdirAll(incomingDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Create dummy test debs in incoming directory
	// Two packages: pkg-alpha_1.0.0_amd64.deb and pkg-beta_1.0.0_amd64.deb
	fAlpha := filepath.Join(incomingDir, "pkg-alpha_1.0.0_amd64.deb")
	fBeta := filepath.Join(incomingDir, "pkg-beta_1.0.0_amd64.deb")

	// Use helper from collector_test or write valid minimal ar deb
	debAlpha := createDummyDebContent("pkg-alpha", "1.0.0", "amd64")
	debBeta := createDummyDebContent("pkg-beta", "1.0.0", "amd64")

	if err := os.WriteFile(fAlpha, debAlpha, 0644); err != nil {
		t.Fatalf("failed writing alpha deb: %v", err)
	}
	if err := os.WriteFile(fBeta, debBeta, 0644); err != nil {
		t.Fatalf("failed writing beta deb: %v", err)
	}

	cfg.Codename = "bookworm"
	cfg.Storage = "file"
	cfg.LocalDir = repoDir
	cfg.Component = "main"

	// Select ONLY pkg-alpha using a glob pattern
	globPattern := filepath.Join(incomingDir, "*alpha*.deb")
	err := publishCmd.RunE(publishCmd, []string{globPattern})
	if err != nil {
		t.Fatalf("publishCmd.RunE failed with glob pattern: %v", err)
	}

	// Verify only pkg-alpha was published to pool and index
	alphaPool := filepath.Join(repoDir, "pool", "main", "p", "pkg-alpha", "pkg-alpha_1.0.0_amd64.deb")
	betaPool := filepath.Join(repoDir, "pool", "main", "p", "pkg-beta", "pkg-beta_1.0.0_amd64.deb")

	if _, err := os.Stat(alphaPool); err != nil {
		t.Errorf("expected alpha package at %s, but stat failed: %v", alphaPool, err)
	}
	if _, err := os.Stat(betaPool); !os.IsNotExist(err) {
		t.Errorf("beta package was NOT selected by glob and should not exist in pool, but was found at %s", betaPool)
	}
}

// Helper to create minimal valid debian package for publish test
func createDummyDebContent(pkgName, version, arch string) []byte {
	var controlTarGz bytes.Buffer
	gzWriter := gzip.NewWriter(&controlTarGz)
	tarWriter := tar.NewWriter(gzWriter)

	controlContent := []byte(strings.Join([]string{
		"Package: " + pkgName,
		"Version: " + version,
		"Architecture: " + arch,
		"Maintainer: Test <test@example.com>",
		"Description: Test package",
		"",
	}, "\n"))

	tarHeader := &tar.Header{
		Name: "./control",
		Mode: 0644,
		Size: int64(len(controlContent)),
	}
	_ = tarWriter.WriteHeader(tarHeader)
	_, _ = tarWriter.Write(controlContent)
	_ = tarWriter.Close()
	_ = gzWriter.Close()

	var debBuf bytes.Buffer
	arWriter := ar.NewWriter(&debBuf)
	_ = arWriter.WriteGlobalHeader()

	debBin := []byte("2.0\n")
	_ = arWriter.WriteHeader(&ar.Header{
		Name: "debian-binary",
		Mode: 0644,
		Size: int64(len(debBin)),
	})
	_, _ = arWriter.Write(debBin)

	_ = arWriter.WriteHeader(&ar.Header{
		Name: "control.tar.gz",
		Mode: 0644,
		Size: int64(controlTarGz.Len()),
	})
	_, _ = arWriter.Write(controlTarGz.Bytes())

	return debBuf.Bytes()
}
