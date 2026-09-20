package debian

import (
	"strings"
	"testing"
)

func TestParseParagraphs_Advanced(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantCount   int
		checkStanza func(t *testing.T, stanzas []*PackageStanza)
		wantErr     bool
	}{
		{
			name: "Multiple paragraphs with multi-line continuation and dot empty line",
			input: `Package: pkg-one
Version: 1.0.0
Architecture: amd64
Maintainer: Test <test@example.com>
Description: First line description
 Extended continuation line
 .
 Next paragraph line in description.
X-Custom-Field: Value1

Package: pkg-two
Version: 2.0.0
Architecture: all
Maintainer: Other <other@example.com>
Description: Second package
`,
			wantCount: 2,
			checkStanza: func(t *testing.T, stanzas []*PackageStanza) {
				s1 := stanzas[0]
				if s1.Package != "pkg-one" {
					t.Errorf("expected pkg-one, got %s", s1.Package)
				}
				if !strings.Contains(s1.Description, "First line description") {
					t.Errorf("missing first line description: %s", s1.Description)
				}
				if !strings.Contains(s1.Description, "Extended continuation line") {
					t.Errorf("missing extended line: %s", s1.Description)
				}
				if !strings.Contains(s1.Description, "Next paragraph line in description.") {
					t.Errorf("missing paragraph line: %s", s1.Description)
				}
				if s1.CustomFields["X-Custom-Field"] != "Value1" {
					t.Errorf("expected X-Custom-Field=Value1, got %s", s1.CustomFields["X-Custom-Field"])
				}

				s2 := stanzas[1]
				if s2.Package != "pkg-two" {
					t.Errorf("expected pkg-two, got %s", s2.Package)
				}
				if s2.Architecture != "all" {
					t.Errorf("expected all arch, got %s", s2.Architecture)
				}
			},
			wantErr: false,
		},
		{
			name:      "Empty and whitespace input",
			input:     "   \n\n\t\t\n   ",
			wantCount: 0,
			checkStanza: func(t *testing.T, stanzas []*PackageStanza) {
				if len(stanzas) != 0 {
					t.Errorf("expected 0 stanzas, got %d", len(stanzas))
				}
			},
			wantErr: false,
		},
		{
			name: "Stanza with numeric size parsing",
			input: `Package: numeric-test
Version: 3.14
Architecture: arm64
Maintainer: Debian Test <debian@test.org>
Size: 987654321
Installed-Size: 123456
Description: Numeric parsing test
`,
			wantCount: 1,
			checkStanza: func(t *testing.T, stanzas []*PackageStanza) {
				s := stanzas[0]
				if s.Size != 987654321 {
					t.Errorf("expected Size 987654321, got %d", s.Size)
				}
				if s.InstalledSize != 123456 {
					t.Errorf("expected InstalledSize 123456, got %d", s.InstalledSize)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paras, err := ParseParagraphs(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseParagraphs() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if len(paras) != tt.wantCount {
				t.Fatalf("ParseParagraphs() count = %d, want %d", len(paras), tt.wantCount)
			}

			var stanzas []*PackageStanza
			for _, p := range paras {
				stanzas = append(stanzas, ParagraphToStanza(p))
			}
			if tt.checkStanza != nil {
				tt.checkStanza(t, stanzas)
			}
		})
	}
}
