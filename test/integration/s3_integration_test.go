//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"debpub/internal/config"
	"debpub/internal/repo"
	"debpub/internal/storage"
)

func getS3Client(ctx context.Context, endpoint, region, accessKey, secretKey string) (*s3.Client, error) {
	customResolver := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(customResolver),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = &endpoint
		o.UsePathStyle = true
	})
	return client, nil
}

func ensureBucket(ctx context.Context, client *s3.Client, bucket string) error {
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &bucket,
	})
	if err != nil {
		// If bucket already exists (BucketAlreadyOwnedByYou), ignore error
		return nil
	}
	return nil
}

func TestS3Backend_MinIO_CRUDAndLocking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	endpoint := getEnvOrDefault("S3_ENDPOINT", "http://minio:9000")
	bucket := getEnvOrDefault("S3_BUCKET", "debian-test")
	region := getEnvOrDefault("AWS_REGION", "us-east-1")
	accessKey := getEnvOrDefault("AWS_ACCESS_KEY_ID", "minioadmin")
	secretKey := getEnvOrDefault("AWS_SECRET_ACCESS_KEY", "minioadminpassword")

	client, err := getS3Client(ctx, endpoint, region, accessKey, secretKey)
	if err != nil {
		t.Fatalf("getS3Client failed: %v", err)
	}

	if err := ensureBucket(ctx, client, bucket); err != nil {
		t.Fatalf("ensureBucket failed: %v", err)
	}

	backend := storage.NewS3Backend(storage.S3Options{
		Client:        client,
		Bucket:        bucket,
		Prefix:        "test-crud",
		LegacyLocking: false,
	})

	t.Run("Put and Exists", func(t *testing.T) {
		data := []byte("minio test payload")
		err := backend.Put(ctx, "test/file.txt", bytes.NewReader(data), int64(len(data)), "text/plain")
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}

		exists, err := backend.Exists(ctx, "test/file.txt")
		if err != nil || !exists {
			t.Fatalf("Exists returned false or error: %v", err)
		}
	})

	t.Run("Get Content", func(t *testing.T) {
		rc, err := backend.Get(ctx, "test/file.txt")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		got := readAllToString(t, rc)
		if got != "minio test payload" {
			t.Fatalf("content mismatch: got %q, want 'minio test payload'", got)
		}
	})

	t.Run("List with Prefix", func(t *testing.T) {
		files, err := backend.List(ctx, "test")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(files) == 0 {
			t.Fatalf("expected files in list, got 0")
		}
	})

	t.Run("Atomic Conditional Lock PutIfNotExist", func(t *testing.T) {
		lockPath := "dists/bookworm/.lock"
		lockPayload := []byte(`{"holder":"test-runner-1","created_at":"2026-09-19T20:00:00Z"}`)

		// First acquisition must succeed
		err := backend.PutIfNotExist(ctx, lockPath, lockPayload)
		if err != nil {
			t.Fatalf("initial PutIfNotExist failed: %v", err)
		}

		// Second concurrent acquisition must fail with ErrAlreadyExists
		secondPayload := []byte(`{"holder":"test-runner-2","created_at":"2026-09-19T20:00:01Z"}`)
		err = backend.PutIfNotExist(ctx, lockPath, secondPayload)
		if !errors.Is(err, storage.ErrAlreadyExists) {
			t.Fatalf("expected ErrAlreadyExists on lock collision, got: %v", err)
		}

		// Cleanup lock
		if err := backend.Delete(ctx, lockPath); err != nil {
			t.Fatalf("Delete lock failed: %v", err)
		}

		exists, _ := backend.Exists(ctx, lockPath)
		if exists {
			t.Fatalf("deleted lock still exists")
		}
	})
}

func TestS3Backend_MinIO_PublisherEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	endpoint := getEnvOrDefault("S3_ENDPOINT", "http://minio:9000")
	bucket := getEnvOrDefault("S3_BUCKET", "debian-test")
	region := getEnvOrDefault("AWS_REGION", "us-east-1")
	accessKey := getEnvOrDefault("AWS_ACCESS_KEY_ID", "minioadmin")
	secretKey := getEnvOrDefault("AWS_SECRET_ACCESS_KEY", "minioadminpassword")

	client, err := getS3Client(ctx, endpoint, region, accessKey, secretKey)
	if err != nil {
		t.Fatalf("getS3Client failed: %v", err)
	}

	prefix := fmt.Sprintf("e2e-s3-%d", time.Now().UnixNano())
	backend := storage.NewS3Backend(storage.S3Options{
		Client:        client,
		Bucket:        bucket,
		Prefix:        prefix,
		LegacyLocking: false,
	})

	cfg := &config.Config{
		Codename:    "bookworm",
		Component:   "main",
		Sign:        false,
		LockEnabled: true,
	}

	publisher := repo.NewPublisher(cfg, backend, nil)

	tempDir := t.TempDir()
	deb1 := writeDebToDisk(t, tempDir, "app-s3", "1.0.0", "amd64")

	t.Run("Publish Initial Package", func(t *testing.T) {
		err := publisher.PublishDebFiles(ctx, []string{deb1})
		if err != nil {
			t.Fatalf("PublishDebFiles failed: %v", err)
		}

		// Verify pool file
		poolPath := "pool/main/a/app-s3/app-s3_1.0.0_amd64.deb"
		exists, err := backend.Exists(ctx, poolPath)
		if err != nil || !exists {
			t.Fatalf("expected pool file %q to exist", poolPath)
		}

		// Verify Packages index contains app-s3
		pkgPath := "dists/bookworm/main/binary-amd64/Packages"
		rc, err := backend.Get(ctx, pkgPath)
		if err != nil {
			t.Fatalf("Get Packages failed: %v", err)
		}
		packagesContent := readAllToString(t, rc)
		if !bytes.Contains([]byte(packagesContent), []byte("Package: app-s3")) {
			t.Fatalf("Packages does not contain 'Package: app-s3':\n%s", packagesContent)
		}

		// Verify Packages.gz and Packages.bz2 exist
		for _, compExt := range []string{".gz", ".bz2", ".xz"} {
			cExists, err := backend.Exists(ctx, pkgPath+compExt)
			if err != nil || !cExists {
				t.Fatalf("expected compressed index %s to exist", pkgPath+compExt)
			}
		}

		// Verify Release manifest exists
		releasePath := "dists/bookworm/Release"
		rExists, err := backend.Exists(ctx, releasePath)
		if err != nil || !rExists {
			t.Fatalf("expected Release manifest to exist")
		}

		// Verify lock is unlocked
		lockPath := "dists/bookworm/.lock"
		lExists, _ := backend.Exists(ctx, lockPath)
		if lExists {
			t.Fatalf("expected .lock to be removed after publish")
		}
	})

	t.Run("Publish Package Upgrade", func(t *testing.T) {
		deb2 := writeDebToDisk(t, tempDir, "app-s3", "1.1.0", "amd64")
		err := publisher.PublishDebFiles(ctx, []string{deb2})
		if err != nil {
			t.Fatalf("PublishDebFiles upgrade failed: %v", err)
		}

		// Verify updated Packages contains version 1.1.0
		pkgPath := "dists/bookworm/main/binary-amd64/Packages"
		rc, err := backend.Get(ctx, pkgPath)
		if err != nil {
			t.Fatalf("Get Packages failed: %v", err)
		}
		packagesContent := readAllToString(t, rc)
		if !bytes.Contains([]byte(packagesContent), []byte("Version: 1.1.0")) {
			t.Fatalf("Packages does not contain updated 'Version: 1.1.0':\n%s", packagesContent)
		}
	})
}
