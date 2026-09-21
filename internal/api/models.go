package api

import (
	"time"
)

// RepoInfo represents metadata and state of the repository being inspected.
type RepoInfo struct {
	Storage        string    `json:"storage"`
	Bucket         string    `json:"bucket,omitempty"`
	Prefix         string    `json:"prefix,omitempty"`
	LocalDir       string    `json:"local_dir,omitempty"`
	Codename       string    `json:"codename"`
	Component      string    `json:"component"`
	Architectures  []string  `json:"architectures"`
	AllCodenames   []string  `json:"all_codenames"`
	AllComponents  []string  `json:"all_components"`
	TotalPackages  int       `json:"total_packages"`
	TotalVersions  int       `json:"total_versions"`
	LastSyncedTime time.Time `json:"last_synced_time"`
	ReleaseDate    string    `json:"release_date,omitempty"`
	Origin         string    `json:"origin,omitempty"`
	Label          string    `json:"label,omitempty"`
	Description    string    `json:"description,omitempty"`
}

// PackageVersionDetail represents full deb822 metadata for a single package version.
type PackageVersionDetail struct {
	Package       string            `json:"package"`
	Version       string            `json:"version"`
	Architecture  string            `json:"architecture"`
	Maintainer    string            `json:"maintainer"`
	InstalledSize int64             `json:"installed_size"`
	Section       string            `json:"section,omitempty"`
	Priority      string            `json:"priority,omitempty"`
	Depends       string            `json:"depends,omitempty"`
	PreDepends    string            `json:"pre_depends,omitempty"`
	Recommends    string            `json:"recommends,omitempty"`
	Suggests      string            `json:"suggests,omitempty"`
	Conflicts     string            `json:"conflicts,omitempty"`
	Breaks        string            `json:"breaks,omitempty"`
	Replaces      string            `json:"replaces,omitempty"`
	Provides      string            `json:"provides,omitempty"`
	Description   string            `json:"description"`
	Homepage      string            `json:"homepage,omitempty"`
	Filename      string            `json:"filename"`
	Size          int64             `json:"size"`
	SHA256        string            `json:"sha256"`
	SHA512        string            `json:"sha512,omitempty"`
	SHA1          string            `json:"sha1,omitempty"`
	MD5sum        string            `json:"md5sum,omitempty"`
	CustomFields  map[string]string `json:"custom_fields,omitempty"`
}

// PackageCard represents a package summary item in the 1-card-per-line list view.
type PackageCard struct {
	Name           string   `json:"name"`
	LatestVersion  string   `json:"latest_version"`
	Architectures  []string `json:"architectures"`
	VersionCount   int      `json:"version_count"`
	Description    string   `json:"description"`
	Maintainer     string   `json:"maintainer"`
	LatestSize     int64    `json:"latest_size"`
	LatestFilename string   `json:"latest_filename"`
	LatestSHA256   string   `json:"latest_sha256"`
}

// PackageDetailResponse represents detailed information for a selected package.
type PackageDetailResponse struct {
	Name     string                 `json:"name"`
	Versions []PackageVersionDetail `json:"versions"`
}

// FetchResponse represents the result of an index fetch/sync operation.
type FetchResponse struct {
	Success         bool      `json:"success"`
	Message         string    `json:"message"`
	Codename        string    `json:"codename"`
	Component       string    `json:"component"`
	PackagesCount   int       `json:"packages_count"`
	SyncedAt        time.Time `json:"synced_at"`
	ExecutionTimeMs int64     `json:"execution_time_ms"`
}
