package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
			"timeout": "2m",
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
	if cfg.LockTimeout != 2*time.Minute {
		t.Errorf("LockTimeout = %v, want 2m", cfg.LockTimeout)
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
