package debian

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// CollectDebFiles resolves a list of input arguments that can contain single files,
// wildcard/glob patterns, or directory paths, returning a sorted and deduplicated
// list of valid Debian package file paths (.deb, .udeb, .ddeb).
func CollectDebFiles(patternsOrPaths []string) ([]string, error) {
	if len(patternsOrPaths) == 0 {
		return nil, errors.New("no files, patterns, or directories specified")
	}

	seen := make(map[string]bool)
	var resolved []string

	for _, input := range patternsOrPaths {
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// 1. Check if argument contains wildcard characters
		if hasGlobMeta(input) {
			matches, err := filepath.Glob(input)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern %q: %w", input, err)
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("no Debian packages found matching pattern %q", input)
			}

			foundValid := false
			for _, match := range matches {
				fi, err := os.Stat(match)
				if err != nil || fi.IsDir() {
					continue
				}
				if isDebPackageFile(match) {
					cleanPath := filepath.Clean(match)
					if !seen[cleanPath] {
						seen[cleanPath] = true
						resolved = append(resolved, cleanPath)
					}
					foundValid = true
				}
			}

			if !foundValid {
				return nil, fmt.Errorf("pattern %q matched files, but none were Debian packages (.deb, .udeb, .ddeb)", input)
			}
			continue
		}

		// 2. Not a wildcard: inspect filesystem path
		fi, err := os.Stat(input)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("file or directory not found: %s", input)
			}
			return nil, fmt.Errorf("unable to access %s: %w", input, err)
		}

		// If it's a directory, walk recursively and collect deb files
		if fi.IsDir() {
			foundInDir := 0
			err = filepath.WalkDir(input, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				if isDebPackageFile(p) {
					cleanPath := filepath.Clean(p)
					if !seen[cleanPath] {
						seen[cleanPath] = true
						resolved = append(resolved, cleanPath)
					}
					foundInDir++
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed walking directory %s: %w", input, err)
			}
			if foundInDir == 0 {
				return nil, fmt.Errorf("no Debian packages found in directory %s", input)
			}
			continue
		}

		// Regular file: verify it is a debian package
		if !isDebPackageFile(input) {
			return nil, fmt.Errorf("file is not a Debian package (.deb, .udeb, .ddeb): %s", input)
		}

		cleanPath := filepath.Clean(input)
		if !seen[cleanPath] {
			seen[cleanPath] = true
			resolved = append(resolved, cleanPath)
		}
	}

	if len(resolved) == 0 {
		return nil, errors.New("no Debian packages (.deb, .udeb, .ddeb) found to publish")
	}

	slices.Sort(resolved)
	return resolved, nil
}

// hasGlobMeta returns true if the path string contains glob wildcard characters (*, ?, [, ]).
func hasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

// isDebPackageFile returns true if the filename has a supported Debian binary package extension.
func isDebPackageFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".deb") ||
		strings.HasSuffix(lower, ".udeb") ||
		strings.HasSuffix(lower, ".ddeb")
}
