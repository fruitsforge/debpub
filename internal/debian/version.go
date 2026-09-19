package debian

import (
	"strconv"
	"strings"
	"unicode"
)

// Version represents a parsed Debian package version: [epoch:]upstream_version[-debian_revision]
type Version struct {
	Epoch    int
	Upstream string
	Revision string
}

// ParseVersion parses a Debian version string according to deb-version(7).
func ParseVersion(v string) Version {
	var ver Version
	parts := strings.SplitN(v, ":", 2)
	var rest string
	if len(parts) == 2 {
		if ep, err := strconv.Atoi(parts[0]); err == nil {
			ver.Epoch = ep
		}
		rest = parts[1]
	} else {
		ver.Epoch = 0
		rest = parts[0]
	}

	revIdx := strings.LastIndex(rest, "-")
	if revIdx != -1 {
		ver.Upstream = rest[:revIdx]
		ver.Revision = rest[revIdx+1:]
	} else {
		ver.Upstream = rest
		ver.Revision = ""
	}

	return ver
}

// CompareVersions compares two Debian version strings conforming to deb-version(7).
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
func CompareVersions(v1, v2 string) int {
	ver1 := ParseVersion(v1)
	ver2 := ParseVersion(v2)

	if ver1.Epoch < ver2.Epoch {
		return -1
	}
	if ver1.Epoch > ver2.Epoch {
		return 1
	}

	upCmp := compareParts(ver1.Upstream, ver2.Upstream)
	if upCmp != 0 {
		return upCmp
	}

	return compareParts(ver1.Revision, ver2.Revision)
}

// compareParts compares two strings according to Debian's modified alphanumeric sort algorithm.
// '~' sorts before everything, even empty string.
func compareParts(s1, s2 string) int {
	i1, i2 := 0, 0
	len1, len2 := len(s1), len(s2)

	for i1 < len1 || i2 < len2 {
		var firstDiff int

		// Non-digits comparison
		for (i1 < len1 && !unicode.IsDigit(rune(s1[i1]))) || (i2 < len2 && !unicode.IsDigit(rune(s2[i2]))) {
			c1 := 0
			if i1 < len1 && !unicode.IsDigit(rune(s1[i1])) {
				c1 = orderChar(s1[i1])
				i1++
			}
			c2 := 0
			if i2 < len2 && !unicode.IsDigit(rune(s2[i2])) {
				c2 = orderChar(s2[i2])
				i2++
			}
			if c1 != c2 {
				return compareInt(c1, c2)
			}
		}

		// Digits comparison
		for i1 < len1 && s1[i1] == '0' {
			i1++
		}
		for i2 < len2 && s2[i2] == '0' {
			i2++
		}

		digits1, digits2 := 0, 0
		for i1 < len1 && unicode.IsDigit(rune(s1[i1])) {
			digits1++
			i1++
		}
		for i2 < len2 && unicode.IsDigit(rune(s2[i2])) {
			digits2++
			i2++
		}

		if digits1 != digits2 {
			return compareInt(digits1, digits2)
		}

		// Numerical comparison when equal length
		start1 := i1 - digits1
		start2 := i2 - digits2
		for k := 0; k < digits1; k++ {
			if s1[start1+k] != s2[start2+k] {
				if firstDiff == 0 {
					firstDiff = compareInt(int(s1[start1+k]), int(s2[start2+k]))
				}
			}
		}
		if firstDiff != 0 {
			return firstDiff
		}
	}

	return 0
}

func orderChar(c byte) int {
	if c == '~' {
		return -1
	}
	if unicode.IsLetter(rune(c)) {
		return int(c)
	}
	if c == 0 {
		return 0
	}
	// Other characters sort after letters
	return int(c) + 256
}

func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
