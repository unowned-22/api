package album

import "context"

type Repository interface {
	Create(ctx context.Context, a *Album) error
	GetByID(ctx context.Context, id int64) (*Album, error)
	ListByUser(ctx context.Context, userID int64, viewerID int64, limit, offset int) ([]*Album, int, error)
	Update(ctx context.Context, a *Album) error
	Delete(ctx context.Context, id int64) error
	SetCover(ctx context.Context, albumID int64, photoID *int64) error
}

type Service interface {
	Create(ctx context.Context, userID int64, input CreateInput) (*Album, error)
	Get(ctx context.Context, id int64, viewerID int64) (*Album, error)
	ListUserAlbums(ctx context.Context, ownerID int64, viewerID int64, limit, offset int) ([]*Album, int, error)
	Update(ctx context.Context, id int64, requesterID int64, input UpdateInput) (*Album, error)
	Delete(ctx context.Context, id int64, requesterID int64) error
	SetCover(ctx context.Context, albumID int64, requesterID int64, photoID *int64) error
}

type CreateInput struct {
	Title       string
	Description string
	Visibility  Visibility
	HiddenFrom  []int64
}

type UpdateInput struct {
	Title       string
	Description string
	Visibility  Visibility
	HiddenFrom  []int64
}
