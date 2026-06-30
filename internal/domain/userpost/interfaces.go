package userpost

import "context"

// Repository persists personal posts.
type Repository interface {
	Create(ctx context.Context, p *Post) error
	GetByID(ctx context.Context, id int64) (*Post, error)
	Update(ctx context.Context, p *Post) error
	SoftDelete(ctx context.Context, id int64) error

	// ListByUser returns a user's own posts (any visibility), newest first.
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*Post, error)

	IncrLikesCount(ctx context.Context, id int64) error
	DecrLikesCount(ctx context.Context, id int64) error
	IncrCommentsCount(ctx context.Context, id int64) error
	DecrCommentsCount(ctx context.Context, id int64) error
}
