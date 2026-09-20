package debian

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// ParseParagraphs reads a stream of RFC 822 / deb822 paragraphs separated by blank lines.
func ParseParagraphs(r io.Reader) ([]*Paragraph, error) {
	scanner := bufio.NewScanner(r)
	var paragraphs []*Paragraph
	current := NewParagraph()
	var currentKey string
	var currentValue strings.Builder

	flushField := func() {
		if currentKey != "" {
			val := strings.TrimRight(currentValue.String(), "\n")
			current.Set(currentKey, val)
			currentKey = ""
			currentValue.Reset()
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Blank line indicates paragraph boundary
		if trimmed == "" {
			flushField()
			if len(current.Order) > 0 {
				paragraphs = append(paragraphs, current)
				current = NewParagraph()
			}
			continue
		}

		// Check for continuation line (starts with space or tab)
		if line[0] == ' ' || line[0] == '\t' {
			if currentKey != "" {
				contLine := strings.TrimPrefix(line, " ")
				contLine = strings.TrimPrefix(contLine, "\t")
				if contLine == "." {
					currentValue.WriteString("\n")
				} else {
					if currentValue.Len() > 0 && !strings.HasSuffix(currentValue.String(), "\n") {
						currentValue.WriteString("\n")
					}
					currentValue.WriteString(contLine)
				}
			}
			continue
		}

		// New field declaration "Key: Value"
		flushField()
		key, val, found := strings.Cut(line, ":")
		if found {
			currentKey = strings.TrimSpace(key)
			currentValue.WriteString(strings.TrimSpace(val))
		}
	}

	flushField()
	if len(current.Order) > 0 {
		paragraphs = append(paragraphs, current)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return paragraphs, nil
}

// ParagraphToStanza converts a deb822 Paragraph into a strongly-typed PackageStanza.
func ParagraphToStanza(p *Paragraph) *PackageStanza {
	s := &PackageStanza{
		Package:      p.Get("Package"),
		Version:      p.Get("Version"),
		Architecture: p.Get("Architecture"),
		Maintainer:   p.Get("Maintainer"),
		Section:      p.Get("Section"),
		Priority:     p.Get("Priority"),
		Essential:    p.Get("Essential"),
		Depends:      p.Get("Depends"),
		PreDepends:   p.Get("Pre-Depends"),
		Recommends:   p.Get("Recommends"),
		Suggests:     p.Get("Suggests"),
		Conflicts:    p.Get("Conflicts"),
		Breaks:       p.Get("Breaks"),
		Replaces:     p.Get("Replaces"),
		Provides:     p.Get("Provides"),
		Description:  p.Get("Description"),
		Homepage:     p.Get("Homepage"),
		CustomFields: make(map[string]string),
		Filename:     p.Get("Filename"),
		SHA256:       p.Get("SHA256"),
		SHA512:       p.Get("SHA512"),
		SHA1:         p.Get("SHA1"),
		MD5sum:       p.Get("MD5sum"),
	}

	if sz, err := strconv.ParseInt(p.Get("Size"), 10, 64); err == nil {
		s.Size = sz
	}
	if sz, err := strconv.ParseInt(p.Get("Installed-Size"), 10, 64); err == nil {
		s.InstalledSize = sz
	}

	standardKeys := map[string]bool{
		"Package": true, "Version": true, "Architecture": true, "Maintainer": true,
		"Installed-Size": true, "Section": true, "Priority": true, "Essential": true,
		"Depends": true, "Pre-Depends": true, "Recommends": true, "Suggests": true,
		"Conflicts": true, "Breaks": true, "Replaces": true, "Provides": true,
		"Description": true, "Homepage": true, "Filename": true, "Size": true,
		"SHA256": true, "SHA512": true, "SHA1": true, "MD5sum": true,
	}

	for _, k := range p.Order {
		if !standardKeys[k] {
			s.CustomFields[k] = p.Get(k)
		}
	}

	return s
}
