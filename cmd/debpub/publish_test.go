package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildStorageBackendS3Options(t *testing.T) {
	tempDir := t.TempDir()

	// Write dummy AWS config and credentials files
	awsConfigFile := filepath.Join(tempDir, "config")
	awsCredsFile := filepath.Join(tempDir, "credentials")

	configContent := `[profile custom_profile]
region = us-west-2
`
	credsContent := `[custom_profile]
aws_access_key_id = TEST_ACCESS_KEY
aws_secret_access_key = TEST_SECRET_KEY
`
	if err := os.WriteFile(awsConfigFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write dummy AWS config: %v", err)
	}
	if err := os.WriteFile(awsCredsFile, []byte(credsContent), 0600); err != nil {
		t.Fatalf("failed to write dummy AWS credentials: %v", err)
	}

	t.Setenv("AWS_CONFIG_FILE", awsConfigFile)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", awsCredsFile)

	ctx := context.Background()

	// Configure cfg with S3 storage, custom profile, and custom region
	cfg.Storage = "s3"
	cfg.Bucket = "test-bucket"
	cfg.S3Profile = "custom_profile"
	cfg.S3Region = "eu-west-1"

	backend, err := buildStorageBackend(ctx)
	if err != nil {
		t.Fatalf("buildStorageBackend failed with custom S3Profile and S3Region: %v", err)
	}
	if backend == nil {
		t.Fatal("expected non-nil StorageBackend")
	}
}
