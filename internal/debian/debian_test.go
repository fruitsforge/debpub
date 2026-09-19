package debian

import (
	"archive/tar"
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/gzip"
)

func TestParseParagraphsAndSerialize(t *testing.T) {
	raw := `Package: my-app
Version: 1.2.3-4
Architecture: amd64
Maintainer: Test User <test@example.com>
Installed-Size: 1024
Section: utils
Priority: optional
Description: First line summary
 Detailed second line.
 .
 Final paragraph line.
Custom-Vendor-Header: FooBar
Filename: pool/main/m/my-app/my-app_1.2.3-4_amd64.deb
Size: 12345
SHA256: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
`
	paras, err := ParseParagraphs(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseParagraphs failed: %v", err)
	}

	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}

	p := paras[0]
	stanza := ParagraphToStanza(p)

	if stanza.Package != "my-app" {
		t.Errorf("Package = %s, want my-app", stanza.Package)
	}
	if stanza.Version != "1.2.3-4" {
		t.Errorf("Version = %s, want 1.2.3-4", stanza.Version)
	}
	if stanza.Size != 12345 {
		t.Errorf("Size = %d, want 12345", stanza.Size)
	}
	if stanza.CustomFields["Custom-Vendor-Header"] != "FooBar" {
		t.Errorf("Custom field Custom-Vendor-Header mismatch: %v", stanza.CustomFields)
	}

	// Verify multiline description preserved
	if !strings.Contains(stanza.Description, "First line summary") || !strings.Contains(stanza.Description, "Final paragraph line.") {
		t.Errorf("Description not parsed properly: %q", stanza.Description)
	}

	serialized := stanza.ToParagraph().String()
	if !strings.Contains(serialized, "Package: my-app") {
		t.Errorf("Serialized output missing Package: %s", serialized)
	}
}

func TestDebianVersionCompare(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0", "1.0", 0},
		{"1.0", "2.0", -1},
		{"2.0", "1.0", 1},
		{"1.0-1", "1.0-2", -1},
		{"1.0.1", "1.0.0", 1},
		{"1:1.0", "2.0", 1},      // Epoch 1 > Epoch 0
		{"1.0~beta1", "1.0", -1}, // Tilde is lower
		{"1.0~beta1", "1.0~beta2", -1},
		{"1.0~~", "1.0~", -1},
		{"1.0-0ubuntu1", "1.0-0ubuntu2", -1},
		{"1.0alpha1", "1.01", -1}, // Numerical 0 < 1
		{"1.0-alpha", "1.0-1", 1}, // Non-digit prefix "-alpha" vs "-"
		{"1.0a", "1.0", 1},        // "a" > empty
		{"1.0", "1.0.0", -1},      // empty < ".0"
		{"2:1.0", "1:2.0", 1},
	}

	for _, tt := range tests {
		cmp := CompareVersions(tt.v1, tt.v2)
		if cmp != tt.expected {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, cmp, tt.expected)
		}
	}
}

func TestIndexAddOrUpdateAndSort(t *testing.T) {
	idx := NewIndex()

	s1 := &PackageStanza{
		PackageControl: PackageControl{
			Package:      "app-b",
			Version:      "1.0.0",
			Architecture: "amd64",
			Description:  "App B",
		},
		Filename: "pool/main/a/app-b/app-b_1.0.0_amd64.deb",
		Size:     100,
	}

	s2 := &PackageStanza{
		PackageControl: PackageControl{
			Package:      "app-a",
			Version:      "1.0.0",
			Architecture: "amd64",
			Description:  "App A",
		},
		Filename: "pool/main/a/app-a/app-a_1.0.0_amd64.deb",
		Size:     100,
	}

	s3 := &PackageStanza{
		PackageControl: PackageControl{
			Package:      "app-a",
			Version:      "2.0.0",
			Architecture: "amd64",
			Description:  "App A v2",
		},
		Filename: "pool/main/a/app-a/app-a_2.0.0_amd64.deb",
		Size:     100,
	}

	// 1. Add without preserveVersions (replaces same package)
	idx.AddOrUpdate(s1, false)
	idx.AddOrUpdate(s2, false)
	idx.AddOrUpdate(s3, false)

	if len(idx.Packages) != 2 {
		t.Fatalf("expected 2 packages without preserveVersions, got %d", len(idx.Packages))
	}

	// Verify app-a was replaced with 2.0.0
	for _, p := range idx.Packages {
		if p.Package == "app-a" && p.Version != "2.0.0" {
			t.Errorf("app-a version = %s, want 2.0.0", p.Version)
		}
	}

	// 2. Add with preserveVersions
	idx2 := NewIndex()
	idx2.AddOrUpdate(s1, true)
	idx2.AddOrUpdate(s2, true)
	idx2.AddOrUpdate(s3, true)

	if len(idx2.Packages) != 3 {
		t.Fatalf("expected 3 packages with preserveVersions, got %d", len(idx2.Packages))
	}

	idx2.Sort()
	if idx2.Packages[0].Package != "app-a" || idx2.Packages[0].Version != "2.0.0" {
		t.Errorf("expected app-a 2.0.0 first, got %s %s", idx2.Packages[0].Package, idx2.Packages[0].Version)
	}
	if idx2.Packages[1].Package != "app-a" || idx2.Packages[1].Version != "1.0.0" {
		t.Errorf("expected app-a 1.0.0 second, got %s %s", idx2.Packages[1].Package, idx2.Packages[1].Version)
	}
	if idx2.Packages[2].Package != "app-b" || idx2.Packages[2].Version != "1.0.0" {
		t.Errorf("expected app-b 1.0.0 third, got %s %s", idx2.Packages[2].Package, idx2.Packages[2].Version)
	}

	serialized := idx2.Serialize()
	if !bytes.Contains(serialized, []byte("Package: app-a")) || !bytes.Contains(serialized, []byte("Package: app-b")) {
		t.Errorf("serialized index missing packages: %s", string(serialized))
	}
}

func TestGenerateReleaseManifest(t *testing.T) {
	manifest := &ReleaseManifest{
		Origin:        "TestOrigin",
		Label:         "TestLabel",
		Codename:      "stable",
		Date:          "Wed, 16 Sep 2026 13:03:17 UTC",
		Architectures: []string{"amd64", "arm64"},
		Components:    []string{"main"},
		Indices: []IndexChecksum{
			{
				Path:   "main/binary-amd64/Packages",
				Size:   1234,
				MD5:    "d41d8cd98f00b204e9800998ecf8427e",
				SHA1:   "da39a3ee5e6b4b0d3255bfef95601890afd80709",
				SHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
		},
	}

	out := GenerateRelease(manifest)
	str := string(out)

	if !strings.Contains(str, "Origin: TestOrigin") {
		t.Errorf("missing Origin")
	}
	if !strings.Contains(str, "Codename: stable") {
		t.Errorf("missing Codename")
	}
	if !strings.Contains(str, "Architectures: amd64 arm64") {
		t.Errorf("missing Architectures")
	}
	if !strings.Contains(str, "SHA256:\n") || !strings.Contains(str, "main/binary-amd64/Packages") {
		t.Errorf("missing SHA256 checksum entry")
	}
}

func TestParseDebReaderStreaming(t *testing.T) {
	// Construct a synthetic .deb archive in memory
	var debBuf bytes.Buffer
	arWriter := ar.NewWriter(&debBuf)
	if err := arWriter.WriteGlobalHeader(); err != nil {
		t.Fatalf("failed writing global ar header: %v", err)
	}
	if err := arWriter.WriteHeader(&ar.Header{
		Name:    "debian-binary",
		Size:    4,
		Mode:    0644,
		ModTime: time.Now(),
	}); err != nil {
		t.Fatalf("failed writing debian-binary header: %v", err)
	}
	if _, err := arWriter.Write([]byte("2.0\n")); err != nil {
		t.Fatalf("failed writing debian-binary content: %v", err)
	}

	// Create control.tar.gz
	var ctarBuf bytes.Buffer
	gw := gzip.NewWriter(&ctarBuf)
	tw := tar.NewWriter(gw)
	controlContent := []byte("Package: test-app\nVersion: 1.2.3\nArchitecture: amd64\nMaintainer: Test <test@example.com>\nDescription: Test App\n")
	if err := tw.WriteHeader(&tar.Header{
		Name:    "./control",
		Size:    int64(len(controlContent)),
		Mode:    0644,
		ModTime: time.Now(),
	}); err != nil {
		t.Fatalf("failed writing tar header: %v", err)
	}
	if _, err := tw.Write(controlContent); err != nil {
		t.Fatalf("failed writing tar content: %v", err)
	}
	_ = tw.Close()
	_ = gw.Close()

	if err := arWriter.WriteHeader(&ar.Header{
		Name:    "control.tar.gz",
		Size:    int64(ctarBuf.Len()),
		Mode:    0644,
		ModTime: time.Now(),
	}); err != nil {
		t.Fatalf("failed writing control.tar.gz header: %v", err)
	}
	if _, err := arWriter.Write(ctarBuf.Bytes()); err != nil {
		t.Fatalf("failed writing control.tar.gz content: %v", err)
	}

	// Create data.tar.gz (to verify streaming continues past control.tar.gz and hashes cover whole archive)
	var dtarBuf bytes.Buffer
	dgw := gzip.NewWriter(&dtarBuf)
	dtw := tar.NewWriter(dgw)
	dataContent := []byte("hello world payload data")
	if err := dtw.WriteHeader(&tar.Header{
		Name:    "./usr/bin/test-app",
		Size:    int64(len(dataContent)),
		Mode:    0755,
		ModTime: time.Now(),
	}); err != nil {
		t.Fatalf("failed writing data tar header: %v", err)
	}
	if _, err := dtw.Write(dataContent); err != nil {
		t.Fatalf("failed writing data tar content: %v", err)
	}
	_ = dtw.Close()
	_ = dgw.Close()

	if err := arWriter.WriteHeader(&ar.Header{
		Name:    "data.tar.gz",
		Size:    int64(dtarBuf.Len()),
		Mode:    0644,
		ModTime: time.Now(),
	}); err != nil {
		t.Fatalf("failed writing data.tar.gz header: %v", err)
	}
	if _, err := arWriter.Write(dtarBuf.Bytes()); err != nil {
		t.Fatalf("failed writing data.tar.gz content: %v", err)
	}

	rawDeb := debBuf.Bytes()
	expectedSize := int64(len(rawDeb))
	expectedMD5 := fmt.Sprintf("%x", md5.Sum(rawDeb))
	expectedSHA1 := fmt.Sprintf("%x", sha1.Sum(rawDeb))
	expectedSHA256 := fmt.Sprintf("%x", sha256.Sum256(rawDeb))
	sha512Bytes := sha512.Sum512(rawDeb)
	expectedSHA512 := hex.EncodeToString(sha512Bytes[:])

	// Parse with streaming ParseDebReader
	pkg, err := ParseDebReader(bytes.NewReader(rawDeb))
	if err != nil {
		t.Fatalf("ParseDebReader failed: %v", err)
	}

	if pkg.Control.Package != "test-app" {
		t.Errorf("Package = %s, want test-app", pkg.Control.Package)
	}
	if pkg.Control.Version != "1.2.3" {
		t.Errorf("Version = %s, want 1.2.3", pkg.Control.Version)
	}
	if pkg.Control.Architecture != "amd64" {
		t.Errorf("Architecture = %s, want amd64", pkg.Control.Architecture)
	}
	if pkg.Size != expectedSize {
		t.Errorf("Size = %d, want %d", pkg.Size, expectedSize)
	}
	if pkg.MD5 != expectedMD5 {
		t.Errorf("MD5 = %s, want %s", pkg.MD5, expectedMD5)
	}
	if pkg.SHA1 != expectedSHA1 {
		t.Errorf("SHA1 = %s, want %s", pkg.SHA1, expectedSHA1)
	}
	if pkg.SHA256 != expectedSHA256 {
		t.Errorf("SHA256 = %s, want %s", pkg.SHA256, expectedSHA256)
	}
	if pkg.SHA512 != expectedSHA512 {
		t.Errorf("SHA512 = %s, want %s", pkg.SHA512, expectedSHA512)
	}
}
