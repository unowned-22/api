package video

import (
	"context"

	mediavideo "github.com/unowned-22/api/internal/domain/media/video"
)

type Repository interface {
	Create(ctx context.Context, v *Video) error
	GetByID(ctx context.Context, id int64) (*Video, error)
	Update(ctx context.Context, v *Video) error
	Delete(ctx context.Context, id int64) error
	ReadyForPublish(ctx context.Context, id int64, hls, mp4360, mp4720, thumbnail string, dur float64, w, h int, size int64, vcodec, acodec string) error
	MarkFailed(ctx context.Context, id int64) error
	MarkProcessing(ctx context.Context, id int64) error
	ListByChannel(ctx context.Context, channelID int64, viewerID int64, limit, offset int) ([]*Video, int, error)
	Feed(ctx context.Context, subscriberID int64, limit, offset int) ([]*Video, int, error)
	Search(ctx context.Context, query, category string, limit, offset int) ([]*Video, int, error)
	SetTags(ctx context.Context, videoID int64, tags []string) error
	GetTags(ctx context.Context, videoID int64) ([]string, error)
	RecordView(ctx context.Context, videoID int64, userID *int64, ipHash string) error
	AddLike(ctx context.Context, userID, videoID int64) error
	RemoveLike(ctx context.Context, userID, videoID int64) error
	IsLiked(ctx context.Context, userID, videoID int64) (bool, error)
	IncrViewsCount(ctx context.Context, id int64) error
	IncrCommentsCount(ctx context.Context, id int64) error
	DecrCommentsCount(ctx context.Context, id int64) error
}

type JobQueue interface {
	Enqueue(ctx context.Context, job mediavideo.ProcessJob) error
}

type Service interface {
	Upload(ctx context.Context, req UploadRequest) (*Video, error)
	GetVideo(ctx context.Context, id int64, viewerID int64) (*Video, error)
	UpdateMeta(ctx context.Context, id int64, requesterID int64, req UpdateMetaRequest) (*Video, error)
	Delete(ctx context.Context, id int64, requesterID int64) error
	ListByChannel(ctx context.Context, channelID int64, viewerID int64, limit, offset int) ([]*Video, int, error)
	Feed(ctx context.Context, subscriberID int64, limit, offset int) ([]*Video, int, error)
	Search(ctx context.Context, query, category string, limit, offset int) ([]*Video, int, error)
	RecordView(ctx context.Context, videoID int64, userID *int64, ipHash string) error
	LikeVideo(ctx context.Context, videoID, userID int64) error
	UnlikeVideo(ctx context.Context, videoID, userID int64) error
}

type UploadRequest struct {
	UserID      int64
	ChannelID   int64
	Title       string
	Description string
	Category    string
	Tags        []string
	Visibility  Visibility
	FileName    string
	ContentType string
	SizeBytes   int64
	Body        interface{ Read([]byte) (int, error) }
}

type UpdateMetaRequest struct {
	Title        string
	Description  string
	Category     string
	Tags         []string
	Visibility   Visibility
	ThumbnailKey string
}
