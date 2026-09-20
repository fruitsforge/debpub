package version

import (
	"strings"
	"testing"
)

func TestHeader(t *testing.T) {
	header := Header()

	if !strings.HasPrefix(header, "debpub ") {
		t.Errorf("expected header to start with 'debpub ', got %q", header)
	}
	if !strings.Contains(header, Version) {
		t.Errorf("expected header to contain Version %q, got %q", Version, header)
	}
	if !strings.Contains(header, GitCommit) {
		t.Errorf("expected header to contain GitCommit %q, got %q", GitCommit, header)
	}
	if !strings.Contains(header, BuildDate) {
		t.Errorf("expected header to contain BuildDate %q, got %q", BuildDate, header)
	}
	if !strings.Contains(header, License) {
		t.Errorf("expected header to contain License %q, got %q", License, header)
	}
	lines := strings.Split(header, "\n")
	if len(lines) != 2 {
		t.Errorf("expected header to have 2 lines, got %d", len(lines))
	}
	if !strings.HasPrefix(lines[1], "License: ") {
		t.Errorf("expected second line to start with 'License: ', got %q", lines[1])
	}
}
