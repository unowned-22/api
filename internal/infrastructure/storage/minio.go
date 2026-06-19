package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
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
	// PublicEndpoint is the externally reachable URL for the storage service
	// used to construct permanent public object URLs (e.g. https://s3.localhost).
	PublicEndpoint string
	// PublicBucket is the bucket name used for public assets (avatars, covers).
	PublicBucket string
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

// BucketExists reports whether the named bucket already exists in MinIO.
// Satisfies domainstorage.Storage and is used by EmailVerifiedHandler to
// make provisioning idempotent without catching SDK-specific error types.
func (s *MinIOStorage) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	exists, err := s.client.BucketExists(ctx, bucketName)
	if err != nil {
		return false, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	return exists, nil
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
func (s *MinIOStorage) ApplyBucketPolicy(ctx context.Context, bucket, policy string) error {
	if s.client == nil {
		return fmt.Errorf("minio client not initialized")
	}
	if err := s.client.SetBucketPolicy(ctx, bucket, policy); err != nil {
		return fmt.Errorf("failed to set bucket policy: %w", err)
	}
	return nil
}

// CreateBucket creates a bucket only if it does not already exist.
func (s *MinIOStorage) CreateBucket(ctx context.Context, bucketName string) error {
	return s.ensureBucketExists(ctx, bucketName)
}

// PutObject uploads an object and returns its presigned GET URL.
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
	// If this object was uploaded into the configured public bucket, return
	// a permanent public URL. For other buckets (private per-user buckets)
	// fallback to generating a presigned URL so callers can access the object.
	if s.config.PublicBucket != "" && bucketName == s.config.PublicBucket {
		base := strings.TrimSuffix(s.config.PublicEndpoint, "/")
		publicURL := fmt.Sprintf("%s/%s/%s", base, bucketName, objectName)
		return publicURL, nil
	}

	// For non-public buckets return a presigned URL (short lived).
	return s.GetURL(ctx, bucketName, objectName, 15*time.Minute)
}

// DeleteObject removes an object from a bucket.
func (s *MinIOStorage) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	return s.Delete(ctx, bucketName, objectName)
}

// PresignURL returns a presigned GET URL for the object.
func (s *MinIOStorage) PresignURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error) {
	return s.GetURL(ctx, bucketName, objectName, expiry)
}
