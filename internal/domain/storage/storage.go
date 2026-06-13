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
