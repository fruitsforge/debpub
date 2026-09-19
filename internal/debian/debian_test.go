package debian

import (
	"bytes"
	"strings"
	"testing"
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
		{"1:1.0", "2.0", 1}, // Epoch 1 > Epoch 0
		{"1.0~beta1", "1.0", -1}, // Tilde is lower
		{"1.0~beta1", "1.0~beta2", -1},
		{"1.0-0ubuntu1", "1.0-0ubuntu2", -1},
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
