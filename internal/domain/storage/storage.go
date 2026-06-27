package storage

import (
	"context"
	"io"
	"time"
)

type ObjectStorage interface {
	Upload(ctx context.Context, req UploadRequest) error
	Delete(ctx context.Context, bucket, key string) error
	GetURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error)
	PresignedPutURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error)
}

type UploadRequest struct {
	Bucket      string
	Key         string
	Body        io.Reader
	Size        int64
	ContentType string
}

type ObjectInfo struct {
	Size        int64
	ContentType string
	Metadata    map[string]string // x-amz-meta-* поля
}

// Storage defines a higher-level storage contract used by services and workers.
type Storage interface {
	// BucketExists reports whether a bucket with the given name already exists.
	// Used by EmailVerifiedHandler to make CreateBucket idempotent without
	// relying on error-type inspection from the MinIO SDK.
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	CreateBucket(ctx context.Context, bucketName string) error
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, contentType string) (string, error)
	DeleteObject(ctx context.Context, bucketName, objectName string) error
	PresignURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error)
	StatObject(ctx context.Context, bucket, key string) (*ObjectInfo, error)
}
