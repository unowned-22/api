package communitypost

import "context"

// Repository persists community-authored posts.
type Repository interface {
	Create(ctx context.Context, p *Post) error
	GetByID(ctx context.Context, id int64) (*Post, error)
	Update(ctx context.Context, p *Post) error
	SoftDelete(ctx context.Context, id int64) error

	// ListByCommunity returns a community's posts, newest first (pinned first).
	ListByCommunity(ctx context.Context, communityID int64, limit, offset int) ([]*Post, error)

	// CreateForVideo is used by the Stage 4 publish bridge — creates a
	// community_posts row referencing a just-published video. Kept as a
	// separate method (rather than overloading Create) so the call site in
	// VideoService.Publish stays a single, explicit, easily-greppable call.
	CreateForVideo(ctx context.Context, communityID, authorUserID, videoID int64) (*Post, error)

	IncrLikesCount(ctx context.Context, id int64) error
	DecrLikesCount(ctx context.Context, id int64) error
	IncrCommentsCount(ctx context.Context, id int64) error
	DecrCommentsCount(ctx context.Context, id int64) error
}
