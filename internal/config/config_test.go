package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Storage != DefaultStorage {
		t.Errorf("Storage = %q, want %q", cfg.Storage, DefaultStorage)
	}
	if cfg.LocalDir != DefaultLocalDir {
		t.Errorf("LocalDir = %q, want %q", cfg.LocalDir, DefaultLocalDir)
	}
	if cfg.Component != DefaultComponent {
		t.Errorf("Component = %q, want %q", cfg.Component, DefaultComponent)
	}
	if cfg.LockEnabled != DefaultLockEnabled {
		t.Errorf("LockEnabled = %v, want %v", cfg.LockEnabled, DefaultLockEnabled)
	}
	if cfg.LockTimeout != DefaultLockTimeout {
		t.Errorf("LockTimeout = %v, want %v", cfg.LockTimeout, DefaultLockTimeout)
	}
	if cfg.LockTTL != DefaultLockTTL {
		t.Errorf("LockTTL = %v, want %v", cfg.LockTTL, DefaultLockTTL)
	}
	if cfg.SFTPPort != DefaultSFTPPort {
		t.Errorf("SFTPPort = %d, want %d", cfg.SFTPPort, DefaultSFTPPort)
	}
	if !cfg.PreserveVersions {
		t.Errorf("PreserveVersions = false, want true")
	}
}

func TestParseDurationFlexible(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"2m", 2 * time.Minute, false},
		{"120s", 2 * time.Minute, false},
		{"120", 2 * time.Minute, false},
		{"3m", 3 * time.Minute, false},
		{"180s", 3 * time.Minute, false},
		{"180", 3 * time.Minute, false},
		{"90", 90 * time.Second, false},
		{"1h", 1 * time.Hour, false},
		{"", 0, false},
		{"   ", 0, false},
		{"-10", 0, true},
		{"-2m", 0, true},
		{"invalid", 0, true},
	}

	for _, tc := range tests {
		got, err := ParseDurationFlexible(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseDurationFlexible(%q) expected error, got nil", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("ParseDurationFlexible(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseDurationFlexible(%q) = %v, want %v", tc.input, got, tc.want)
			}
		}
	}
}

func TestFormatDurationDisplay(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{2 * time.Minute, "2m"},
		{3 * time.Minute, "3m"},
		{120 * time.Second, "2m"},
		{45 * time.Second, "45s"},
		{0, "0s"},
		{90 * time.Second, "90s"},
	}

	for _, tc := range tests {
		got := FormatDurationDisplay(tc.d)
		if got != tc.want {
			t.Errorf("FormatDurationDisplay(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestLoadConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "debpub.json")

	jsonContent := `{
		"storage": "s3",
		"bucket": "my-apt-repo",
		"prefix": "deb",
		"codename": "bookworm",
		"component": "contrib",
		"architectures": ["amd64", "arm64"],
		"preserve_versions": true,
		"origin": "CustomOrigin",
		"label": "CustomLabel",
		"gpg": {
			"sign": true,
			"key": "1234ABCD",
			"passphrase": "secret"
		},
		"lock": {
			"enabled": true,
			"timeout": "4m",
			"ttl": "8m"
		},
		"s3": {
			"endpoint": "http://minio:9000",
			"force_path_style": true,
			"legacy_locking": false,
			"profile": "staging",
			"region": "eu-central-1"
		},
		"metadata": {
			"pipeline_id": "999"
		}
	}`

	if err := os.WriteFile(configPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write test config file: %v", err)
	}

	cfg := DefaultConfig()
	if err := LoadConfigFile(configPath, cfg); err != nil {
		t.Fatalf("LoadConfigFile failed: %v", err)
	}

	if cfg.Storage != "s3" {
		t.Errorf("Storage = %s, want s3", cfg.Storage)
	}
	if cfg.Bucket != "my-apt-repo" {
		t.Errorf("Bucket = %s, want my-apt-repo", cfg.Bucket)
	}
	if cfg.Codename != "bookworm" {
		t.Errorf("Codename = %s, want bookworm", cfg.Codename)
	}
	if cfg.Component != "contrib" {
		t.Errorf("Component = %s, want contrib", cfg.Component)
	}
	if !cfg.PreserveVersions {
		t.Errorf("PreserveVersions = false, want true")
	}
	if !cfg.Sign || cfg.GPGKey != "1234ABCD" {
		t.Errorf("GPG sign/key mismatch: %v, %s", cfg.Sign, cfg.GPGKey)
	}
	if cfg.LockTimeout != 4*time.Minute {
		t.Errorf("LockTimeout = %v, want 4m", cfg.LockTimeout)
	}
	if cfg.LockTTL != 8*time.Minute {
		t.Errorf("LockTTL = %v, want 8m", cfg.LockTTL)
	}
	if cfg.S3Endpoint != "http://minio:9000" || !cfg.S3ForcePathStyle {
		t.Errorf("S3 settings mismatch: %v, %v", cfg.S3Endpoint, cfg.S3ForcePathStyle)
	}
	if cfg.S3Profile != "staging" {
		t.Errorf("S3Profile = %s, want staging", cfg.S3Profile)
	}
	if cfg.S3Region != "eu-central-1" {
		t.Errorf("S3Region = %s, want eu-central-1", cfg.S3Region)
	}
	if cfg.ExtraMetadata["pipeline_id"] != "999" {
		t.Errorf("Metadata mismatch: %v", cfg.ExtraMetadata)
	}
}

func TestLoadConfigFile_NumericAndRawSeconds(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "debpub.json")

	// Test with numeric JSON number and raw string integer
	jsonContent := `{
		"codename": "jammy",
		"lock": {
			"timeout": 120,
			"ttl": "180"
		}
	}`

	if err := os.WriteFile(configPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write test config file: %v", err)
	}

	cfg := DefaultConfig()
	if err := LoadConfigFile(configPath, cfg); err != nil {
		t.Fatalf("LoadConfigFile failed: %v", err)
	}

	if cfg.LockTimeout != 2*time.Minute {
		t.Errorf("LockTimeout = %v, want 2m (from 120)", cfg.LockTimeout)
	}
	if cfg.LockTTL != 3*time.Minute {
		t.Errorf("LockTTL = %v, want 3m (from \"180\")", cfg.LockTTL)
	}
	// Omitted fields retain defaults
	if cfg.Storage != DefaultStorage {
		t.Errorf("Storage = %q, want default %q", cfg.Storage, DefaultStorage)
	}
	if cfg.Component != DefaultComponent {
		t.Errorf("Component = %q, want default %q", cfg.Component, DefaultComponent)
	}
}

func TestLoadConfigFile_OmittedLockRetainsDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "debpub.json")

	jsonContent := `{"codename": "bookworm"}`

	if err := os.WriteFile(configPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write test config file: %v", err)
	}

	cfg := DefaultConfig()
	if err := LoadConfigFile(configPath, cfg); err != nil {
		t.Fatalf("LoadConfigFile failed: %v", err)
	}

	if cfg.LockTimeout != DefaultLockTimeout {
		t.Errorf("LockTimeout = %v, want default %v", cfg.LockTimeout, DefaultLockTimeout)
	}
	if cfg.LockTTL != DefaultLockTTL {
		t.Errorf("LockTTL = %v, want default %v", cfg.LockTTL, DefaultLockTTL)
	}
}
