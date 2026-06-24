package photocomment

import "context"

type Repository interface {
	Create(ctx context.Context, c *Comment) error
	GetByID(ctx context.Context, id int64) (*Comment, error)
	ListRoots(ctx context.Context, photoID int64, viewerID int64, limit, offset int) ([]*Comment, int, error)
	ListReplies(ctx context.Context, parentID int64, viewerID int64, limit, offset int) ([]*Comment, int, error)
	SoftDelete(ctx context.Context, id int64) error
	Update(ctx context.Context, id int64, body string) error
	AddLike(ctx context.Context, userID, commentID int64) error
	RemoveLike(ctx context.Context, userID, commentID int64) error
	IsLiked(ctx context.Context, userID, commentID int64) (bool, error)
}

type Service interface {
	AddComment(ctx context.Context, photoID int64, authorID int64, input AddCommentInput) (*Comment, error)
	GetComment(ctx context.Context, id int64, viewerID int64) (*Comment, error)
	ListComments(ctx context.Context, photoID int64, viewerID int64, limit, offset int) ([]*Comment, int, error)
	ListReplies(ctx context.Context, parentID int64, viewerID int64, limit, offset int) ([]*Comment, int, error)
	EditComment(ctx context.Context, id int64, requesterID int64, body string) (*Comment, error)
	DeleteComment(ctx context.Context, id int64, requesterID int64) error
	LikeComment(ctx context.Context, commentID int64, userID int64) error
	UnlikeComment(ctx context.Context, commentID int64, userID int64) error
}

type AddCommentInput struct {
	ParentID *int64
	Body     string
}
