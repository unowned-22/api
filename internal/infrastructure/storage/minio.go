package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	domainstorage "github.com/unowned-22/api/internal/domain/storage"
)

type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	Region          string
}

type MinIOStorage struct {
	client *minio.Client
	config Config
}

var _ domainstorage.ObjectStorage = (*MinIOStorage)(nil)
var _ domainstorage.Storage = (*MinIOStorage)(nil)

func NewMinIOStorage(cfg Config) (*MinIOStorage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MinIO client: %w", err)
	}

	return &MinIOStorage{client: client, config: cfg}, nil
}

func (s *MinIOStorage) CreateBucketIfNotExists(ctx context.Context, bucket string) error {
	return s.ensureBucketExists(ctx, bucket)
}

func (s *MinIOStorage) ensureBucketExists(ctx context.Context, bucket string) error {
	exists, err := s.client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: s.config.Region}); err != nil {
		return fmt.Errorf("failed to create bucket %q: %w", bucket, err)
	}
	return nil
}

func (s *MinIOStorage) Upload(ctx context.Context, req domainstorage.UploadRequest) error {
	if err := s.ensureBucketExists(ctx, req.Bucket); err != nil {
		return err
	}

	_, err := s.client.PutObject(ctx, req.Bucket, req.Key, req.Body, req.Size, minio.PutObjectOptions{ContentType: req.ContentType})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}
	return nil
}

func (s *MinIOStorage) Delete(ctx context.Context, bucket, key string) error {
	if err := s.ensureBucketExists(ctx, bucket); err != nil {
		return err
	}
	if err := s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("failed to remove object: %w", err)
	}
	return nil
}

func (s *MinIOStorage) GetURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	if err := s.ensureBucketExists(ctx, bucket); err != nil {
		return "", err
	}

	reqParams := make(url.Values)
	url, err := s.client.PresignedGetObject(ctx, bucket, key, expires, reqParams)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned get URL: %w", err)
	}
	return url.String(), nil
}

func (s *MinIOStorage) PresignedPutURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	if err := s.ensureBucketExists(ctx, bucket); err != nil {
		return "", err
	}
	url, err := s.client.PresignedPutObject(ctx, bucket, key, expires)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned put URL: %w", err)
	}
	return url.String(), nil
}

// ApplyBucketPolicy attempts to set a bucket policy using the MinIO client.
// If the underlying SDK exposes SetBucketPolicy, this will apply the policy; otherwise it's a no-op.
func (s *MinIOStorage) ApplyBucketPolicy(ctx context.Context, bucket, policy string) error {
	if s.client == nil {
		return fmt.Errorf("minio client not initialized")
	}
	// minio.Client provides SetBucketPolicy in supported versions.
	// Attempt to call it; if it fails, return the error so callers can decide to retry.
	if err := s.client.SetBucketPolicy(ctx, bucket, policy); err != nil {
		return fmt.Errorf("failed to set bucket policy: %w", err)
	}
	return nil
}

// CreateBucket creates a bucket (no-op if already exists).
func (s *MinIOStorage) CreateBucket(ctx context.Context, bucketName string) error {
	return s.ensureBucketExists(ctx, bucketName)
}

// PutObject uploads an object and returns its URL.
func (s *MinIOStorage) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, contentType string) (string, error) {
	req := domainstorage.UploadRequest{
		Bucket:      bucketName,
		Key:         objectName,
		Body:        reader,
		Size:        size,
		ContentType: contentType,
	}
	if err := s.Upload(ctx, req); err != nil {
		return "", err
	}
	// Return presigned GET URL with short expiry
	url, err := s.GetURL(ctx, bucketName, objectName, 15*time.Minute)
	if err != nil {
		return "", err
	}
	return url, nil
}

// DeleteObject removes an object from a bucket.
func (s *MinIOStorage) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	return s.Delete(ctx, bucketName, objectName)
}

// PresignURL returns a presigned GET URL for the object.
func (s *MinIOStorage) PresignURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error) {
	return s.GetURL(ctx, bucketName, objectName, expiry)
}
