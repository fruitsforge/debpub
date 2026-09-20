package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Default configuration constants.
const (
	DefaultStorage          = "file"
	DefaultLocalDir         = "."
	DefaultComponent        = "main"
	DefaultPreserveVersions = true
	DefaultLockEnabled      = true
	DefaultLockTimeout      = 2 * time.Minute
	DefaultLockTTL          = 3 * time.Minute
	DefaultSFTPPort         = 22
)

// ParseDurationFlexible parses a duration from string supporting both:
// - Standard Go duration strings with unit suffixes (e.g. "2m", "120s", "3m", "90s")
// - Raw integer seconds without unit suffixes (e.g. "120", "180", "90")
func ParseDurationFlexible(val string) (time.Duration, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, nil
	}

	// Check if the input is a pure integer (in seconds)
	if sec, err := strconv.ParseInt(val, 10, 64); err == nil {
		if sec < 0 {
			return 0, fmt.Errorf("duration cannot be negative: %d", sec)
		}
		return time.Duration(sec) * time.Second, nil
	}

	// Otherwise parse using standard time.ParseDuration
	d, err := time.ParseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (use units like '2m', '120s' or raw seconds like '120'): %w", val, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("duration cannot be negative: %v", d)
	}
	return d, nil
}

// FormatDurationDisplay returns a clean, user-friendly duration representation (e.g. "2m", "120s", "3m").
func FormatDurationDisplay(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d%time.Second == 0 {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return d.String()
}
