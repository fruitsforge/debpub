package testutil

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blakesmith/ar"
)

// DebOptions defines customizable parameters for generating test .deb packages.
type DebOptions struct {
	Package      string
	Version      string
	Architecture string
	Maintainer   string
	Description  string
	CustomFields map[string]string
	DataFiles    map[string][]byte
}

// CreateTestDeb creates a valid Debian binary package archive in memory.
func CreateTestDeb(t *testing.T, opts DebOptions) []byte {
	if t != nil {
		t.Helper()
	}

	if opts.Maintainer == "" {
		opts.Maintainer = "Debpub Test Team <test@example.com>"
	}
	if opts.Description == "" {
		opts.Description = "Test package for " + opts.Package
	}

	// 1. Build control file contents
	var controlLines []string
	controlLines = append(controlLines,
		"Package: "+opts.Package,
		"Version: "+opts.Version,
		"Architecture: "+opts.Architecture,
		"Maintainer: "+opts.Maintainer,
		"Description: "+opts.Description,
	)

	for k, v := range opts.CustomFields {
		controlLines = append(controlLines, fmt.Sprintf("%s: %s", k, v))
	}
	controlContent := []byte(strings.Join(controlLines, "\n") + "\n")

	// 2. Wrap control file in control.tar.gz
	var controlTarGz bytes.Buffer
	gw := gzip.NewWriter(&controlTarGz)
	tw := tar.NewWriter(gw)

	tarHeader := &tar.Header{
		Name:    "./control",
		Mode:    0644,
		Size:    int64(len(controlContent)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(tarHeader); err != nil {
		t.Fatalf("testutil: failed writing tar header: %v", err)
	}
	if _, err := tw.Write(controlContent); err != nil {
		t.Fatalf("testutil: failed writing control content: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("testutil: failed closing tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("testutil: failed closing gzip writer: %v", err)
	}

	// 3. Optional data.tar.gz
	var dataTarGz bytes.Buffer
	dgw := gzip.NewWriter(&dataTarGz)
	dtw := tar.NewWriter(dgw)

	for filename, content := range opts.DataFiles {
		dHeader := &tar.Header{
			Name:    filename,
			Mode:    0755,
			Size:    int64(len(content)),
			ModTime: time.Now(),
		}
		if err := dtw.WriteHeader(dHeader); err != nil {
			t.Fatalf("testutil: failed writing data tar header: %v", err)
		}
		if _, err := dtw.Write(content); err != nil {
			t.Fatalf("testutil: failed writing data tar content: %v", err)
		}
	}
	if err := dtw.Close(); err != nil {
		t.Fatalf("testutil: failed closing data tar writer: %v", err)
	}
	if err := dgw.Close(); err != nil {
		t.Fatalf("testutil: failed closing data gzip writer: %v", err)
	}

	// 4. Assemble ar archive
	var debBuf bytes.Buffer
	arWriter := ar.NewWriter(&debBuf)
	if err := arWriter.WriteGlobalHeader(); err != nil {
		t.Fatalf("testutil: failed writing global ar header: %v", err)
	}

	// 4a. debian-binary
	debBin := []byte("2.0\n")
	if err := arWriter.WriteHeader(&ar.Header{
		Name:    "debian-binary",
		Mode:    0644,
		Size:    int64(len(debBin)),
		ModTime: time.Now(),
	}); err != nil {
		t.Fatalf("testutil: failed writing debian-binary header: %v", err)
	}
	if _, err := arWriter.Write(debBin); err != nil {
		t.Fatalf("testutil: failed writing debian-binary content: %v", err)
	}

	// 4b. control.tar.gz
	if err := arWriter.WriteHeader(&ar.Header{
		Name:    "control.tar.gz",
		Mode:    0644,
		Size:    int64(controlTarGz.Len()),
		ModTime: time.Now(),
	}); err != nil {
		t.Fatalf("testutil: failed writing control.tar.gz header: %v", err)
	}
	if _, err := arWriter.Write(controlTarGz.Bytes()); err != nil {
		t.Fatalf("testutil: failed writing control.tar.gz content: %v", err)
	}

	// 4c. data.tar.gz
	if err := arWriter.WriteHeader(&ar.Header{
		Name:    "data.tar.gz",
		Mode:    0644,
		Size:    int64(dataTarGz.Len()),
		ModTime: time.Now(),
	}); err != nil {
		t.Fatalf("testutil: failed writing data.tar.gz header: %v", err)
	}
	if _, err := arWriter.Write(dataTarGz.Bytes()); err != nil {
		t.Fatalf("testutil: failed writing data.tar.gz content: %v", err)
	}

	return debBuf.Bytes()
}

// WriteTestDebToDisk generates a .deb package and writes it to disk, returning the absolute file path.
func WriteTestDebToDisk(t *testing.T, dir, pkgName, version, arch string) string {
	t.Helper()
	data := CreateTestDeb(t, DebOptions{
		Package:      pkgName,
		Version:      version,
		Architecture: arch,
	})
	filePath := filepath.Join(dir, fmt.Sprintf("%s_%s_%s.deb", pkgName, version, arch))
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("testutil: failed writing test deb to disk: %v", err)
	}
	return filePath
}
