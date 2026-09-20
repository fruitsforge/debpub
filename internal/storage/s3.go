package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3Backend implements StorageBackend for AWS S3 and S3-compatible endpoints.
type S3Backend struct {
	client        *s3.Client
	bucket        string
	prefix        string
	legacyLocking bool
}

// S3Options configures S3Backend initialization.
type S3Options struct {
	Client        *s3.Client
	Bucket        string
	Prefix        string
	LegacyLocking bool
}

// NewS3Backend creates a new S3Backend.
func NewS3Backend(opts S3Options) *S3Backend {
	return &S3Backend{
		client:        opts.Client,
		bucket:        opts.Bucket,
		prefix:        strings.Trim(opts.Prefix, "/"),
		legacyLocking: opts.LegacyLocking,
	}
}

func (s *S3Backend) key(path string) string {
	clean := strings.TrimPrefix(path, "/")
	if s.prefix == "" {
		return clean
	}
	return s.prefix + "/" + clean
}

// Get retrieves an object reader from AWS S3 storage.
func (s *S3Backend) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	key := s.key(path)
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if _, ok := errors.AsType[*s3types.NoSuchKey](err); ok {
			return nil, ErrNotFound
		}
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("s3 get: %w", err)
	}
	return output.Body, nil
}

// Put writes an object to AWS S3 storage.
func (s *S3Backend) Put(ctx context.Context, path string, data io.Reader, size int64, contentType string) error {
	key := s.key(path)
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   data,
	}
	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("s3 put: %w", err)
	}
	return nil
}

// PutBytes is a convenience helper storing a raw byte slice in AWS S3.
func (s *S3Backend) PutBytes(ctx context.Context, path string, data []byte, contentType string) error {
	return s.Put(ctx, path, bytes.NewReader(data), int64(len(data)), contentType)
}

// Delete removes an object from AWS S3 storage.
func (s *S3Backend) Delete(ctx context.Context, path string) error {
	key := s.key(path)
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
}

// Exists checks if an object exists in AWS S3 storage.
func (s *S3Backend) Exists(ctx context.Context, path string) (bool, error) {
	key := s.key(path)
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}

	var respErr *awshttp.ResponseError
	if errors.As(err, &respErr) && respErr.HTTPStatusCode() == 404 {
		return false, nil
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey") {
		return false, nil
	}

	return false, fmt.Errorf("s3 exists head: %w", err)
}

// List returns relative object paths matching a prefix in AWS S3 storage.
func (s *S3Backend) List(ctx context.Context, prefix string) ([]string, error) {
	fullPrefix := s.key(prefix)
	var keys []string

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(fullPrefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("s3 list: %w", err)
		}
		for _, obj := range page.Contents {
			relKey := *obj.Key
			if s.prefix != "" {
				relKey = strings.TrimPrefix(relKey, s.prefix+"/")
			}
			keys = append(keys, relKey)
		}
	}

	return keys, nil
}

// PutIfNotExist uses S3 conditional write (If-None-Match: "*") with legacy fallback.
func (s *S3Backend) PutIfNotExist(ctx context.Context, path string, data []byte) error {
	key := s.key(path)

	if !s.legacyLocking {
		// Attempt native S3 conditional write
		input := &s3.PutObjectInput{
			Bucket:      aws.String(s.bucket),
			Key:         aws.String(key),
			Body:        bytes.NewReader(data),
			IfNoneMatch: aws.String("*"),
		}
		_, err := s.client.PutObject(ctx, input)
		if err == nil {
			return nil
		}

		// Check if PreconditionFailed (HTTP 412)
		var respErr *awshttp.ResponseError
		if errors.As(err, &respErr) && respErr.HTTPStatusCode() == 412 {
			return ErrAlreadyExists
		}
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "PreconditionFailed" {
			return ErrAlreadyExists
		}

		// If server returned 400 or 501 (e.g. older MinIO or Ceph not supporting If-None-Match), fallback
		if errors.As(err, &respErr) && (respErr.HTTPStatusCode() == 400 || respErr.HTTPStatusCode() == 501) {
			// Fallback to legacy optimistic check-then-put
			return s.putIfNotExistLegacy(ctx, path, data)
		}

		return fmt.Errorf("s3 conditional put: %w", err)
	}

	return s.putIfNotExistLegacy(ctx, path, data)
}

func (s *S3Backend) putIfNotExistLegacy(ctx context.Context, path string, data []byte) error {
	exists, err := s.Exists(ctx, path)
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyExists
	}

	return s.PutBytes(ctx, path, data, "application/json")
}
