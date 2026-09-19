package debian

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCollectDebFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Setup directory structure
	// tempDir/
	//   pkg1_1.0.0_amd64.deb
	//   pkg2_2.0.0_all.udeb
	//   ignored.txt
	//   nested/
	//     pkg3_3.0.0_arm64.deb
	//     nested2/
	//       pkg4_4.0.0_amd64.ddeb
	//       notes.md
	//   empty_dir/

	f1 := filepath.Join(tempDir, "pkg1_1.0.0_amd64.deb")
	f2 := filepath.Join(tempDir, "pkg2_2.0.0_all.udeb")
	txt := filepath.Join(tempDir, "ignored.txt")

	nestedDir := filepath.Join(tempDir, "nested")
	nested2Dir := filepath.Join(nestedDir, "nested2")
	emptyDir := filepath.Join(tempDir, "empty_dir")

	if err := os.MkdirAll(nested2Dir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("mkdir emptyDir failed: %v", err)
	}

	f3 := filepath.Join(nestedDir, "pkg3_3.0.0_arm64.deb")
	f4 := filepath.Join(nested2Dir, "pkg4_4.0.0_amd64.ddeb")
	md := filepath.Join(nested2Dir, "notes.md")

	for _, file := range []string{f1, f2, txt, f3, f4, md} {
		if err := os.WriteFile(file, []byte("dummy deb binary content"), 0644); err != nil {
			t.Fatalf("write file %s failed: %v", file, err)
		}
	}

	tests := []struct {
		name      string
		inputs    []string
		wantFiles []string
		wantErr   bool
	}{
		{
			name:      "Single explicit deb file",
			inputs:    []string{f1},
			wantFiles: []string{f1},
			wantErr:   false,
		},
		{
			name:      "Multiple explicit deb files with duplicate",
			inputs:    []string{f1, f2, f1},
			wantFiles: []string{f1, f2},
			wantErr:   false,
		},
		{
			name:      "Glob pattern matching root packages",
			inputs:    []string{filepath.Join(tempDir, "pkg*.deb")},
			wantFiles: []string{f1},
			wantErr:   false,
		},
		{
			name:      "Glob pattern matching multiple deb types",
			inputs:    []string{filepath.Join(tempDir, "pkg*.*deb")},
			wantFiles: []string{f1, f2},
			wantErr:   false,
		},
		{
			name:      "Recursive directory walk",
			inputs:    []string{nestedDir},
			wantFiles: []string{f3, f4},
			wantErr:   false,
		},
		{
			name:      "Entire tempDir directory walk",
			inputs:    []string{tempDir},
			wantFiles: []string{f1, f2, f3, f4},
			wantErr:   false,
		},
		{
			name:      "Non-existent file",
			inputs:    []string{filepath.Join(tempDir, "non-existent.deb")},
			wantFiles: nil,
			wantErr:   true,
		},
		{
			name:      "Non-deb regular file",
			inputs:    []string{txt},
			wantFiles: nil,
			wantErr:   true,
		},
		{
			name:      "Empty directory without packages",
			inputs:    []string{emptyDir},
			wantFiles: nil,
			wantErr:   true,
		},
		{
			name:      "Glob with no matches",
			inputs:    []string{filepath.Join(tempDir, "nomatch*.deb")},
			wantFiles: nil,
			wantErr:   true,
		},
		{
			name:      "Glob matching non-deb files only",
			inputs:    []string{filepath.Join(tempDir, "*.txt")},
			wantFiles: nil,
			wantErr:   true,
		},
		{
			name:      "Glob character class selection [1-2]",
			inputs:    []string{filepath.Join(tempDir, "pkg[1-2]_*")},
			wantFiles: []string{f1, f2},
			wantErr:   false,
		},
		{
			name:      "Glob question mark single char selection",
			inputs:    []string{filepath.Join(tempDir, "pkg?_*")},
			wantFiles: []string{f1, f2},
			wantErr:   false,
		},
		{
			name:      "Glob specific package selection [3]",
			inputs:    []string{filepath.Join(nestedDir, "pkg[3]_*.deb")},
			wantFiles: []string{f3},
			wantErr:   false,
		},
		{
			name:      "Glob with leading whitespace trimmed",
			inputs:    []string{"  " + filepath.Join(tempDir, "pkg1_*.deb") + "  "},
			wantFiles: []string{f1},
			wantErr:   false,
		},
		{
			name:      "Empty inputs slice",
			inputs:    []string{},
			wantFiles: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CollectDebFiles(tt.inputs)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CollectDebFiles() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if len(got) != len(tt.wantFiles) {
					t.Fatalf("CollectDebFiles() len = %d (%v), want %d (%v)", len(got), got, len(tt.wantFiles), tt.wantFiles)
				}
				for i, path := range tt.wantFiles {
					if !slices.Contains(got, path) {
						t.Errorf("expected result to contain %q at %d, but got %v", path, i, got)
					}
				}
			}
		})
	}
}
