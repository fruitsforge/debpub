package debian

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"
)

// Index represents an in-memory Debian Packages index file.
type Index struct {
	Packages []*PackageStanza
}

// NewIndex creates a new empty Index.
func NewIndex() *Index {
	return &Index{
		Packages: make([]*PackageStanza, 0),
	}
}

// ParseIndex parses a Packages index from raw deb822 text data.
func ParseIndex(data []byte) (*Index, error) {
	idx := NewIndex()
	if len(bytes.TrimSpace(data)) == 0 {
		return idx, nil
	}

	paras, err := ParseParagraphs(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed parsing Packages index: %w", err)
	}

	for _, p := range paras {
		stanza := ParagraphToStanza(p)
		idx.Packages = append(idx.Packages, stanza)
	}

	return idx, nil
}

// AddOrUpdate adds or updates a package in the index according to preserveVersions.
// If preserveVersions is false, any older version of the same package is replaced.
// If preserveVersions is true, the package is added alongside existing versions unless the exact version already exists.
func (idx *Index) AddOrUpdate(stanza *PackageStanza, preserveVersions bool) {
	if preserveVersions {
		for i, existing := range idx.Packages {
			if existing.Package == stanza.Package && existing.Architecture == stanza.Architecture && existing.Version == stanza.Version {
				idx.Packages[i] = stanza
				return
			}
		}
		idx.Packages = append(idx.Packages, stanza)
		return
	}

	// Not preserving versions: replace package with same name and architecture
	replaced := false
	var updated []*PackageStanza
	for _, existing := range idx.Packages {
		if existing.Package == stanza.Package && existing.Architecture == stanza.Architecture {
			if !replaced {
				updated = append(updated, stanza)
				replaced = true
			}
		} else {
			updated = append(updated, existing)
		}
	}

	if !replaced {
		updated = append(updated, stanza)
	}
	idx.Packages = updated
}

// Remove removes a package by name (and optionally version) from the index.
func (idx *Index) Remove(pkgName, version string) bool {
	var kept []*PackageStanza
	removed := false

	for _, existing := range idx.Packages {
		if existing.Package == pkgName {
			if version == "" || existing.Version == version {
				removed = true
				continue
			}
		}
		kept = append(kept, existing)
	}

	idx.Packages = kept
	return removed
}

// Sort sorts the packages in the index alphabetically by Package name, then descending by Debian Version (latest first), then ascending by Architecture.
func (idx *Index) Sort() {
	slices.SortFunc(idx.Packages, func(a, b *PackageStanza) int {
		if c := cmp.Compare(a.Package, b.Package); c != 0 {
			return c
		}
		// Descending by Debian Version (latest first): Compare b to a
		if c := CompareVersions(b.Version, a.Version); c != 0 {
			return c
		}
		return cmp.Compare(a.Architecture, b.Architecture)
	})
}

// Serialize renders the Packages index into canonical deb822 text format.
func (idx *Index) Serialize() []byte {
	idx.Sort()
	var buf bytes.Buffer
	for _, stanza := range idx.Packages {
		para := stanza.ToParagraph()
		buf.WriteString(para.String())
		buf.WriteString("\n")
	}
	return buf.Bytes()
}
