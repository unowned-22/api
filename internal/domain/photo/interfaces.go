package photo

import (
	"context"
	"io"
)

type Repository interface {
	Create(ctx context.Context, p *Photo) error
	GetByID(ctx context.Context, id int64) (*Photo, error)
	ListByUser(ctx context.Context, userID int64, viewerID int64, limit, offset int) ([]*Photo, int, error)
	ListByAlbum(ctx context.Context, albumID int64, viewerID int64, limit, offset int) ([]*Photo, int, error)
	UpdateMeta(ctx context.Context, id int64, displayName string, visibility Visibility, hiddenFrom []int64) error
	MoveToAlbum(ctx context.Context, id int64, albumID *int64) error
	Delete(ctx context.Context, id int64) error
	GetByStorageKey(ctx context.Context, key string) (*Photo, error)
}

type Service interface {
	Upload(ctx context.Context, userID int64, input UploadInput) (*Photo, error)
	GetPhoto(ctx context.Context, id int64, viewerID int64) (*Photo, error)
	ListUserPhotos(ctx context.Context, ownerID int64, viewerID int64, limit, offset int) ([]*Photo, int, error)
	ListAlbumPhotos(ctx context.Context, albumID int64, viewerID int64, limit, offset int) ([]*Photo, int, error)
	UpdateMeta(ctx context.Context, id int64, requesterID int64, displayName string, visibility Visibility, hiddenFrom []int64) error
	MoveToAlbum(ctx context.Context, id int64, requesterID int64, albumID *int64) error
	Delete(ctx context.Context, id int64, requesterID int64) error
}

type UploadInput struct {
	Reader      io.Reader
	Size        int64
	Filename    string
	ContentType string
	AlbumID     *int64
}
