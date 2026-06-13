package storage

import (
	"context"
	"io"
	"time"
)

// ObjectStorage — контракт для хранения файлов.
// Реализация живёт в internal/infrastructure/storage.
type ObjectStorage interface {
	// Upload загружает файл из reader в указанный bucket/key.
	Upload(ctx context.Context, req UploadRequest) error

	// Delete удаляет объект по bucket/key.
	Delete(ctx context.Context, bucket, key string) error

	// GetURL возвращает прямую (публичную или presigned) URL объекта.
	GetURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error)

	// PresignedPutURL генерирует presigned URL для прямой загрузки с клиента.
	PresignedPutURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error)
}

// UploadRequest содержит данные для загрузки объекта.
type UploadRequest struct {
	Bucket      string
	Key         string // путь внутри bucket, например "avatars/user-42.png"
	Body        io.Reader
	Size        int64
	ContentType string
}

// Storage defines a higher-level storage contract used by services and workers.
type Storage interface {
	CreateBucket(ctx context.Context, bucketName string) error
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, contentType string) (string, error)
	DeleteObject(ctx context.Context, bucketName, objectName string) error
	PresignURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error)
}
