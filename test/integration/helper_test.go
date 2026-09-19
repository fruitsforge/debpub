//go:build integration

package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blakesmith/ar"
)

// Helper to construct a valid minimal in-memory .deb file
func createTestDeb(t *testing.T, pkgName, version, arch, description string) []byte {
	t.Helper()

	controlContent := []byte(strings.Join([]string{
		"Package: " + pkgName,
		"Version: " + version,
		"Architecture: " + arch,
		"Maintainer: Debpub Test Team <test@example.com>",
		"Description: " + description,
		"",
	}, "\n"))

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
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("failed closing tar writer: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("failed closing gzip writer: %v", err)
	}

	var debBuf bytes.Buffer
	arWriter := ar.NewWriter(&debBuf)
	if err := arWriter.WriteGlobalHeader(); err != nil {
		t.Fatalf("WriteGlobalHeader failed: %v", err)
	}

	debianBinaryContent := []byte("2.0\n")
	if err := arWriter.WriteHeader(&ar.Header{
		Name: "debian-binary",
		Mode: 0644,
		Size: int64(len(debianBinaryContent)),
	}); err != nil {
		t.Fatalf("WriteHeader debian-binary failed: %v", err)
	}
	if _, err := arWriter.Write(debianBinaryContent); err != nil {
		t.Fatalf("Write debian-binary failed: %v", err)
	}

	if err := arWriter.WriteHeader(&ar.Header{
		Name: "control.tar.gz",
		Mode: 0644,
		Size: int64(controlTarGz.Len()),
	}); err != nil {
		t.Fatalf("WriteHeader control.tar.gz failed: %v", err)
	}
	if _, err := arWriter.Write(controlTarGz.Bytes()); err != nil {
		t.Fatalf("Write control.tar.gz failed: %v", err)
	}

	var dataTarGz bytes.Buffer
	dGz := gzip.NewWriter(&dataTarGz)
	dTar := tar.NewWriter(dGz)
	_ = dTar.Close()
	_ = dGz.Close()

	if err := arWriter.WriteHeader(&ar.Header{
		Name: "data.tar.gz",
		Mode: 0644,
		Size: int64(dataTarGz.Len()),
	}); err != nil {
		t.Fatalf("WriteHeader data.tar.gz failed: %v", err)
	}
	if _, err := arWriter.Write(dataTarGz.Bytes()); err != nil {
		t.Fatalf("Write data.tar.gz failed: %v", err)
	}

	return debBuf.Bytes()
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
