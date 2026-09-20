package debian

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"
)

// GenerateRelease builds the Release file string content from metadata and index checksums.
//
// noinspection GoUnhandledErrorResult
//
//nolint:errcheck // Buffer writes never fail
func GenerateRelease(m *ReleaseManifest) []byte {
	var buf bytes.Buffer

	if m.Origin != "" {
		fmt.Fprintf(&buf, "Origin: %s\n", m.Origin)
	}
	if m.Label != "" {
		fmt.Fprintf(&buf, "Label: %s\n", m.Label)
	}
	if m.Suite != "" {
		fmt.Fprintf(&buf, "Suite: %s\n", m.Suite)
	}
	fmt.Fprintf(&buf, "Codename: %s\n", m.Codename)

	dateStr := m.Date
	if dateStr == "" {
		dateStr = time.Now().UTC().Format(time.RFC1123)
	}
	fmt.Fprintf(&buf, "Date: %s\n", dateStr)

	if m.ValidUntil != "" {
		fmt.Fprintf(&buf, "Valid-Until: %s\n", m.ValidUntil)
	}

	if len(m.Architectures) > 0 {
		archs := dedupeStrings(m.Architectures)
		slices.Sort(archs)
		fmt.Fprintf(&buf, "Architectures: %s\n", strings.Join(archs, " "))
	}

	if len(m.Components) > 0 {
		comps := dedupeStrings(m.Components)
		slices.Sort(comps)
		fmt.Fprintf(&buf, "Components: %s\n", strings.Join(comps, " "))
	}

	if m.Description != "" {
		fmt.Fprintf(&buf, "Description: %s\n", m.Description)
	}

	// Sort indices deterministically by relative path
	slices.SortFunc(m.Indices, func(a, b IndexChecksum) int {
		return cmp.Compare(a.Path, b.Path)
	})

	// MD5sum table
	buf.WriteString("MD5Sum:\n")
	for _, entry := range m.Indices {
		if entry.MD5 != "" {
			fmt.Fprintf(&buf, " %s %16d %s\n", entry.MD5, entry.Size, entry.Path)
		}
	}

	// SHA1 table
	buf.WriteString("SHA1:\n")
	for _, entry := range m.Indices {
		if entry.SHA1 != "" {
			fmt.Fprintf(&buf, " %s %16d %s\n", entry.SHA1, entry.Size, entry.Path)
		}
	}

	// SHA256 table
	buf.WriteString("SHA256:\n")
	for _, entry := range m.Indices {
		if entry.SHA256 != "" {
			fmt.Fprintf(&buf, " %s %16d %s\n", entry.SHA256, entry.Size, entry.Path)
		}
	}

	// SHA512 table
	buf.WriteString("SHA512:\n")
	for _, entry := range m.Indices {
		if entry.SHA512 != "" {
			fmt.Fprintf(&buf, " %s %16d %s\n", entry.SHA512, entry.Size, entry.Path)
		}
	}

	return buf.Bytes()
}

func dedupeStrings(s []string) []string {
	seen := make(map[string]bool, len(s))
	res := make([]string, 0, len(s))
	for _, v := range s {
		if v != "" && !seen[v] {
			seen[v] = true
			res = append(res, v)
		}
	}
	return res
}
