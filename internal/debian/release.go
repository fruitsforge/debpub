package debian

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"
)

// GenerateRelease builds the Release file string content from metadata and index checksums.
func GenerateRelease(m *ReleaseManifest) []byte {
	var buf bytes.Buffer

	if m.Origin != "" {
		buf.WriteString(fmt.Sprintf("Origin: %s\n", m.Origin))
	}
	if m.Label != "" {
		buf.WriteString(fmt.Sprintf("Label: %s\n", m.Label))
	}
	if m.Suite != "" {
		buf.WriteString(fmt.Sprintf("Suite: %s\n", m.Suite))
	}
	buf.WriteString(fmt.Sprintf("Codename: %s\n", m.Codename))

	dateStr := m.Date
	if dateStr == "" {
		dateStr = time.Now().UTC().Format(time.RFC1123)
	}
	buf.WriteString(fmt.Sprintf("Date: %s\n", dateStr))

	if m.ValidUntil != "" {
		buf.WriteString(fmt.Sprintf("Valid-Until: %s\n", m.ValidUntil))
	}

	if len(m.Architectures) > 0 {
		archs := dedupeStrings(m.Architectures)
		sort.Strings(archs)
		buf.WriteString(fmt.Sprintf("Architectures: %s\n", strings.Join(archs, " ")))
	}

	if len(m.Components) > 0 {
		comps := dedupeStrings(m.Components)
		sort.Strings(comps)
		buf.WriteString(fmt.Sprintf("Components: %s\n", strings.Join(comps, " ")))
	}

	if m.Description != "" {
		buf.WriteString(fmt.Sprintf("Description: %s\n", m.Description))
	}

	// Sort indices deterministically by relative path
	sort.Slice(m.Indices, func(i, j int) bool {
		return m.Indices[i].Path < m.Indices[j].Path
	})

	// MD5sum table
	buf.WriteString("MD5Sum:\n")
	for _, entry := range m.Indices {
		if entry.MD5 != "" {
			buf.WriteString(fmt.Sprintf(" %s %16d %s\n", entry.MD5, entry.Size, entry.Path))
		}
	}

	// SHA1 table
	buf.WriteString("SHA1:\n")
	for _, entry := range m.Indices {
		if entry.SHA1 != "" {
			buf.WriteString(fmt.Sprintf(" %s %16d %s\n", entry.SHA1, entry.Size, entry.Path))
		}
	}

	// SHA256 table
	buf.WriteString("SHA256:\n")
	for _, entry := range m.Indices {
		if entry.SHA256 != "" {
			buf.WriteString(fmt.Sprintf(" %s %16d %s\n", entry.SHA256, entry.Size, entry.Path))
		}
	}

	// SHA512 table
	buf.WriteString("SHA512:\n")
	for _, entry := range m.Indices {
		if entry.SHA512 != "" {
			buf.WriteString(fmt.Sprintf(" %s %16d %s\n", entry.SHA512, entry.Size, entry.Path))
		}
	}

	return buf.Bytes()
}

func dedupeStrings(s []string) []string {
	seen := make(map[string]bool)
	var res []string
	for _, v := range s {
		if v != "" && !seen[v] {
			seen[v] = true
			res = append(res, v)
		}
	}
	return res
}
