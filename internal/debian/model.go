package debian

import (
	"fmt"
	"strconv"
	"strings"
)

// Paragraph represents raw RFC 822 key-value pairs preserving original field ordering.
type Paragraph struct {
	Order  []string
	Fields map[string]string
}

// NewParagraph creates a new empty Paragraph.
func NewParagraph() *Paragraph {
	return &Paragraph{
		Order:  make([]string, 0),
		Fields: make(map[string]string),
	}
}

// Set sets a key-value pair, appending to Order if key is not yet present.
func (p *Paragraph) Set(key, value string) {
	if _, exists := p.Fields[key]; !exists {
		p.Order = append(p.Order, key)
	}
	p.Fields[key] = value
}

// Get returns the value of key or empty string if not present.
func (p *Paragraph) Get(key string) string {
	return p.Fields[key]
}

// PackageControl holds the parsed binary control file specification (deb-control(5)).
type PackageControl struct {
	Package       string            `deb822:"Package"`
	Version       string            `deb822:"Version"`
	Architecture  string            `deb822:"Architecture"`
	Maintainer    string            `deb822:"Maintainer"`
	InstalledSize int64             `deb822:"Installed-Size,omitempty"`
	Section       string            `deb822:"Section,omitempty"`
	Priority      string            `deb822:"Priority,omitempty"`
	Essential     string            `deb822:"Essential,omitempty"`
	Depends       string            `deb822:"Depends,omitempty"`
	PreDepends    string            `deb822:"Pre-Depends,omitempty"`
	Recommends    string            `deb822:"Recommends,omitempty"`
	Suggests      string            `deb822:"Suggests,omitempty"`
	Conflicts     string            `deb822:"Conflicts,omitempty"`
	Breaks        string            `deb822:"Breaks,omitempty"`
	Replaces      string            `deb822:"Replaces,omitempty"`
	Provides      string            `deb822:"Provides,omitempty"`
	Description   string            `deb822:"Description"`
	Homepage      string            `deb822:"Homepage,omitempty"`
	CustomFields  map[string]string `deb822:",extra"` // Preserves custom/vendor X-* fields
}

// PackageStanza represents a complete entry in a Debian Packages index file.
type PackageStanza struct {
	PackageControl

	// Repository-managed fields (added during repository indexing)
	Filename string `deb822:"Filename"`
	Size     int64  `deb822:"Size"`
	SHA256   string `deb822:"SHA256"`
	SHA512   string `deb822:"SHA512,omitempty"`
	SHA1     string `deb822:"SHA1,omitempty"`
	MD5sum   string `deb822:"MD5sum,omitempty"`
}

// IndexChecksum holds a file record in the Release manifest checksum tables.
type IndexChecksum struct {
	Path   string
	Size   int64
	MD5    string
	SHA1   string
	SHA256 string
	SHA512 string
}

// ReleaseManifest represents the dists/<codename>/Release specification.
type ReleaseManifest struct {
	Origin        string          `deb822:"Origin,omitempty"`
	Label         string          `deb822:"Label,omitempty"`
	Suite         string          `deb822:"Suite,omitempty"`
	Codename      string          `deb822:"Codename"`
	Date          string          `deb822:"Date"`
	ValidUntil    string          `deb822:"Valid-Until,omitempty"`
	Architectures []string        `deb822:"Architectures"`
	Components    []string        `deb822:"Components"`
	Description   string          `deb822:"Description,omitempty"`
	Indices       []IndexChecksum `deb822:",checksums"`
}

// ToParagraph converts a PackageStanza into a Debian RFC 822 Paragraph.
func (s *PackageStanza) ToParagraph() *Paragraph {
	p := NewParagraph()
	p.Set("Package", s.Package)
	p.Set("Version", s.Version)
	p.Set("Architecture", s.Architecture)
	p.Set("Maintainer", s.Maintainer)
	if s.InstalledSize > 0 {
		p.Set("Installed-Size", strconv.FormatInt(s.InstalledSize, 10))
	}
	if s.Section != "" {
		p.Set("Section", s.Section)
	}
	if s.Priority != "" {
		p.Set("Priority", s.Priority)
	}
	if s.Essential != "" {
		p.Set("Essential", s.Essential)
	}
	if s.Depends != "" {
		p.Set("Depends", s.Depends)
	}
	if s.PreDepends != "" {
		p.Set("Pre-Depends", s.PreDepends)
	}
	if s.Recommends != "" {
		p.Set("Recommends", s.Recommends)
	}
	if s.Suggests != "" {
		p.Set("Suggests", s.Suggests)
	}
	if s.Conflicts != "" {
		p.Set("Conflicts", s.Conflicts)
	}
	if s.Breaks != "" {
		p.Set("Breaks", s.Breaks)
	}
	if s.Replaces != "" {
		p.Set("Replaces", s.Replaces)
	}
	if s.Provides != "" {
		p.Set("Provides", s.Provides)
	}
	if s.Homepage != "" {
		p.Set("Homepage", s.Homepage)
	}
	for k, v := range s.CustomFields {
		p.Set(k, v)
	}
	p.Set("Description", s.Description)
	p.Set("Filename", s.Filename)
	p.Set("Size", strconv.FormatInt(s.Size, 10))
	p.Set("SHA256", s.SHA256)
	if s.SHA512 != "" {
		p.Set("SHA512", s.SHA512)
	}
	if s.SHA1 != "" {
		p.Set("SHA1", s.SHA1)
	}
	if s.MD5sum != "" {
		p.Set("MD5sum", s.MD5sum)
	}
	return p
}

// Format deb822 string output for a Paragraph
func (p *Paragraph) String() string {
	var sb strings.Builder
	for _, key := range p.Order {
		val := p.Fields[key]
		lines := strings.Split(val, "\n")
		if len(lines) == 1 {
			fmt.Fprintf(&sb, "%s: %s\n", key, val)
		} else {
			fmt.Fprintf(&sb, "%s:\n", key)
			for _, line := range lines {
				switch {
				case line == "":
					sb.WriteString(" .\n")
				case strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t"):
					sb.WriteString(line + "\n")
				default:
					sb.WriteString(" " + line + "\n")
				}
			}
		}
	}
	return sb.String()
}
