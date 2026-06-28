package videocomment

import "context"

type Repository interface {
	Create(ctx context.Context, c *Comment) error
	GetByID(ctx context.Context, id int64) (*Comment, error)
	ListByVideo(ctx context.Context, videoID int64, limit, offset int) ([]*Comment, int, error)
	ListReplies(ctx context.Context, parentID int64) ([]*Comment, error)
	SoftDelete(ctx context.Context, id int64) error
	AddLike(ctx context.Context, userID, commentID int64) error
	RemoveLike(ctx context.Context, userID, commentID int64) error
	IsLiked(ctx context.Context, userID, commentID int64) (bool, error)
}

type Service interface {
	AddComment(ctx context.Context, videoID, userID int64, parentID *int64, body string) (*Comment, error)
	DeleteComment(ctx context.Context, id int64, requesterID int64) error
	ListComments(ctx context.Context, videoID int64, viewerID int64, limit, offset int) ([]*Comment, int, error)
	ListReplies(ctx context.Context, parentID int64, viewerID int64) ([]*Comment, error)
	LikeComment(ctx context.Context, commentID, userID int64) error
	UnlikeComment(ctx context.Context, commentID, userID int64) error
}
