// Package version manages version information, build metadata, and banners for debpub.
package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic release version. Can be overridden via -ldflags="-X debpub/internal/version.Version=v0.0.1".
	Version = "0.0.1"

	// GitCommit is the git SHA injected during build.
	GitCommit = "none"

	// BuildDate is the UTC build timestamp injected during build.
	BuildDate = "unknown"

	// License notice.
	License = "Apache License, Version 2.0"
)

// Header returns the standard banner string: Version on line 1, License on line 2.
func Header() string {
	return fmt.Sprintf("debpub %s (%s, %s, %s/%s)\nLicense: %s",
		Version, GitCommit, BuildDate, runtime.GOOS, runtime.GOARCH, License)
}
