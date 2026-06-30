package communitypost

import "context"

type Repository interface {
	Create(ctx context.Context, p *Post) error
	GetByID(ctx context.Context, id int64) (*Post, error)
	Update(ctx context.Context, p *Post) error
	SoftDelete(ctx context.Context, id int64) error
	ListByCommunity(ctx context.Context, communityID int64, limit, offset int) ([]*Post, error)
	CreateForVideo(ctx context.Context, communityID, authorUserID, videoID int64) (*Post, error)
	SoftDeleteByVideoID(ctx context.Context, videoID int64) (*int64, error)
	IncrLikesCount(ctx context.Context, id int64) error
	DecrLikesCount(ctx context.Context, id int64) error
	IncrCommentsCount(ctx context.Context, id int64) error
	DecrCommentsCount(ctx context.Context, id int64) error
}
