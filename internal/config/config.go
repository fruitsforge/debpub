// Package config manages configuration loading, validation, and defaults for debpub.
package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"time"
)

// Config represents the complete debpub runtime configuration.
type Config struct {
	Storage          string            `json:"storage"`
	Bucket           string            `json:"bucket"`
	Prefix           string            `json:"prefix"`
	Codename         string            `json:"codename"`
	Component        string            `json:"component"`
	Architectures    []string          `json:"architectures"`
	PreserveVersions bool              `json:"preserve_versions"`
	Origin           string            `json:"origin"`
	Label            string            `json:"label"`
	Suite            string            `json:"suite"`
	Description      string            `json:"description"`
	Sign             bool              `json:"sign"`
	GPGKey           string            `json:"gpg_key"`
	GPGPassphrase    string            `json:"gpg_passphrase"`
	GPGExtraArgs     []string          `json:"gpg_extra_args"`
	LockEnabled      bool              `json:"lock_enabled"`
	LockTimeout      time.Duration     `json:"lock_timeout"`
	LockTTL          time.Duration     `json:"lock_ttl"`
	S3Endpoint       string            `json:"s3_endpoint"`
	S3ForcePathStyle bool              `json:"s3_force_path_style"`
	S3LegacyLocking  bool              `json:"s3_legacy_locking"`
	S3Profile        string            `json:"s3_profile"`
	S3Region         string            `json:"s3_region"`
	SFTPHost         string            `json:"sftp_host"`
	SFTPPort         int               `json:"sftp_port"`
	SFTPUser         string            `json:"sftp_user"`
	SFTPPassword     string            `json:"sftp_password"`
	SFTPKeyPath      string            `json:"sftp_key_path"`
	LocalDir         string            `json:"local_dir"`
	ExtraMetadata    map[string]string `json:"extra_metadata"`
}

// ConfigFileSchema represents the JSON structure for debpub.json.
type ConfigFileSchema struct {
	Storage          string            `json:"storage"`
	Bucket           string            `json:"bucket"`
	Prefix           string            `json:"prefix"`
	Codename         string            `json:"codename"`
	Component        string            `json:"component"`
	Architectures    []string          `json:"architectures"`
	PreserveVersions *bool             `json:"preserve_versions"`
	Origin           string            `json:"origin"`
	Label            string            `json:"label"`
	Suite            string            `json:"suite"`
	Description      string            `json:"description"`
	LocalDir         string            `json:"local_dir"`
	GPG              *GPGConfig        `json:"gpg"`
	Lock             *LockConfig       `json:"lock"`
	S3               *S3Config         `json:"s3"`
	SFTP             *SFTPConfig       `json:"sftp"`
	Metadata         map[string]string `json:"metadata"`
}

// GPGConfig holds GPG signing settings in configuration files.
type GPGConfig struct {
	Sign       *bool    `json:"sign"`
	Key        string   `json:"key"`
	Passphrase string   `json:"passphrase"`
	ExtraArgs  []string `json:"extra_args"`
}

// LockConfig holds repository locking settings in configuration files.
// Timeout and TTL accept both duration strings (e.g. "2m", "120s", "120") and numeric seconds (e.g. 120).
type LockConfig struct {
	Enabled *bool `json:"enabled"`
	Timeout any   `json:"timeout"`
	TTL     any   `json:"ttl"`
}

// S3Config holds AWS S3 storage parameters in configuration files.
type S3Config struct {
	Endpoint       string `json:"endpoint"`
	ForcePathStyle *bool  `json:"force_path_style"`
	LegacyLocking  *bool  `json:"legacy_locking"`
	Profile        string `json:"profile"`
	Region         string `json:"region"`
}

// SFTPConfig holds remote SFTP storage parameters in configuration files.
type SFTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	KeyPath  string `json:"key_path"`
}

// DefaultConfig returns baseline configuration defaults.
func DefaultConfig() *Config {
	return &Config{
		Storage:          DefaultStorage,
		LocalDir:         DefaultLocalDir,
		Component:        DefaultComponent,
		LockEnabled:      DefaultLockEnabled,
		LockTimeout:      DefaultLockTimeout,
		LockTTL:          DefaultLockTTL,
		SFTPPort:         DefaultSFTPPort,
		PreserveVersions: DefaultPreserveVersions,
		ExtraMetadata:    make(map[string]string),
	}
}

// LoadConfigFile loads and merges a debpub.json configuration into target Config.
func LoadConfigFile(path string, target *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config: cannot read config file %q: %w", path, err)
	}

	var schema ConfigFileSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return fmt.Errorf("config: invalid JSON in config file %q: %w", path, err)
	}

	if schema.Storage != "" {
		target.Storage = schema.Storage
	}
	if schema.Bucket != "" {
		target.Bucket = schema.Bucket
	}
	if schema.Prefix != "" {
		target.Prefix = schema.Prefix
	}
	if schema.Codename != "" {
		target.Codename = schema.Codename
	}
	if schema.Component != "" {
		target.Component = schema.Component
	}
	if len(schema.Architectures) > 0 {
		target.Architectures = schema.Architectures
	}
	if schema.PreserveVersions != nil {
		target.PreserveVersions = *schema.PreserveVersions
	}
	if schema.Origin != "" {
		target.Origin = schema.Origin
	}
	if schema.Label != "" {
		target.Label = schema.Label
	}
	if schema.Suite != "" {
		target.Suite = schema.Suite
	}
	if schema.Description != "" {
		target.Description = schema.Description
	}
	if schema.LocalDir != "" {
		target.LocalDir = schema.LocalDir
	}

	if schema.GPG != nil {
		if schema.GPG.Sign != nil {
			target.Sign = *schema.GPG.Sign
		}
		if schema.GPG.Key != "" {
			target.GPGKey = schema.GPG.Key
		}
		if schema.GPG.Passphrase != "" {
			target.GPGPassphrase = schema.GPG.Passphrase
		}
		if len(schema.GPG.ExtraArgs) > 0 {
			target.GPGExtraArgs = schema.GPG.ExtraArgs
		}
	}

	if schema.Lock != nil {
		if schema.Lock.Enabled != nil {
			target.LockEnabled = *schema.Lock.Enabled
		}
		if schema.Lock.Timeout != nil {
			if d, err := parseJSONDuration(schema.Lock.Timeout); err == nil && d > 0 {
				target.LockTimeout = d
			}
		}
		if schema.Lock.TTL != nil {
			if d, err := parseJSONDuration(schema.Lock.TTL); err == nil && d > 0 {
				target.LockTTL = d
			}
		}
	}

	if schema.S3 != nil {
		if schema.S3.Endpoint != "" {
			target.S3Endpoint = schema.S3.Endpoint
		}
		if schema.S3.ForcePathStyle != nil {
			target.S3ForcePathStyle = *schema.S3.ForcePathStyle
		}
		if schema.S3.LegacyLocking != nil {
			target.S3LegacyLocking = *schema.S3.LegacyLocking
		}
		if schema.S3.Profile != "" {
			target.S3Profile = schema.S3.Profile
		}
		if schema.S3.Region != "" {
			target.S3Region = schema.S3.Region
		}
	}

	if schema.SFTP != nil {
		if schema.SFTP.Host != "" {
			target.SFTPHost = schema.SFTP.Host
		}
		if schema.SFTP.Port != 0 {
			target.SFTPPort = schema.SFTP.Port
		}
		if schema.SFTP.User != "" {
			target.SFTPUser = schema.SFTP.User
		}
		if schema.SFTP.Password != "" {
			target.SFTPPassword = schema.SFTP.Password
		}
		if schema.SFTP.KeyPath != "" {
			target.SFTPKeyPath = schema.SFTP.KeyPath
		}
	}

	maps.Copy(target.ExtraMetadata, schema.Metadata)

	return nil
}

func parseJSONDuration(val any) (time.Duration, error) {
	switch v := val.(type) {
	case string:
		return ParseDurationFlexible(v)
	case float64:
		if v < 0 {
			return 0, fmt.Errorf("duration cannot be negative: %v", v)
		}
		return time.Duration(v) * time.Second, nil
	case int:
		if v < 0 {
			return 0, fmt.Errorf("duration cannot be negative: %d", v)
		}
		return time.Duration(v) * time.Second, nil
	default:
		return 0, fmt.Errorf("unexpected duration type: %T", val)
	}
}
