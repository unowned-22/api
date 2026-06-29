package service

import (
	"context"
	"fmt"
	"io"
	"path"

	"github.com/google/uuid"
	"github.com/unowned-22/api/internal/domain/event"
	mediavideo "github.com/unowned-22/api/internal/domain/media/video"
	"github.com/unowned-22/api/internal/domain/storage"
	domainvideo "github.com/unowned-22/api/internal/domain/video"
	"github.com/unowned-22/api/internal/domain/videochannel"
	"github.com/unowned-22/api/internal/errs"
)

type VideoService struct {
	videoRepo   domainvideo.Repository
	channelRepo videochannel.Repository
	storage     storage.Storage
	jobQueue    domainvideo.JobQueue
	bucket      string
	publisher   event.Publisher
	maxFileSize int64
}

func NewVideoService(v domainvideo.Repository, c videochannel.Repository, s storage.Storage, q domainvideo.JobQueue, bucket string, pub event.Publisher, maxFileSize int64) *VideoService {
	return &VideoService{videoRepo: v, channelRepo: c, storage: s, jobQueue: q, bucket: bucket, publisher: pub, maxFileSize: maxFileSize}
}

func (s *VideoService) Upload(ctx context.Context, req domainvideo.UploadRequest) (*domainvideo.Video, error) {
	if req.SizeBytes > s.maxFileSize {
		return nil, errs.ErrAttachmentTooLarge
	}
	if req.ContentType != "video/mp4" && req.ContentType != "video/webm" && req.ContentType != "video/quicktime" {
		return nil, errs.ErrUnsupportedVideoType
	}
	key := path.Join("videos", fmt.Sprintf("%d", req.ChannelID), "raw", uuid.NewString(), req.FileName)
	if _, err := s.storage.PutObject(ctx, s.bucket, key, req.Body.(io.Reader), req.SizeBytes, req.ContentType); err != nil {
		return nil, err
	}
	v := &domainvideo.Video{UserID: req.UserID, ChannelID: req.ChannelID, Title: req.Title, Description: req.Description, Category: req.Category, Tags: req.Tags, Visibility: req.Visibility, Status: domainvideo.StatusPending, RawKey: key, SizeBytes: req.SizeBytes}
	if err := s.videoRepo.Create(ctx, v); err != nil {
		return nil, err
	}
	if err := s.videoRepo.SetTags(ctx, v.ID, req.Tags); err != nil {
		return nil, err
	}
	if err := s.channelRepo.IncrVideosCount(ctx, req.ChannelID); err != nil {
		return nil, err
	}
	_ = s.jobQueue.Enqueue(ctx, mediavideo.ProcessJob{UserID: req.UserID, VideoID: v.ID, ChannelID: req.ChannelID, RawKey: key})
	return v, nil
}
func (s *VideoService) GetVideo(ctx context.Context, id int64, viewerID int64) (*domainvideo.Video, error) {
	v, err := s.videoRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if v.Visibility == domainvideo.VisibilityPrivate && v.UserID != viewerID {
		return nil, errs.ErrVideoNotFound
	}

	if viewerID != 0 {
		liked, err := s.videoRepo.IsLiked(ctx, viewerID, id)
		if err == nil {
			v.IsLiked = liked
		}
	}

	return v, nil
}
func (s *VideoService) UpdateMeta(ctx context.Context, id int64, requesterID int64, req domainvideo.UpdateMetaRequest) (*domainvideo.Video, error) {
	v, err := s.videoRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if v.UserID != requesterID {
		return nil, errs.ErrVideoNotOwned
	}
	v.Title, v.Description, v.Category, v.Visibility, v.ThumbnailKey = req.Title, req.Description, req.Category, req.Visibility, req.ThumbnailKey
	v.CoverKey = v.ThumbnailKey
	if err := s.videoRepo.Update(ctx, v); err != nil {
		return nil, err
	}
	if len(req.Tags) > 0 {
		_ = s.videoRepo.SetTags(ctx, id, req.Tags)
	}
	return v, nil
}
func (s *VideoService) Delete(ctx context.Context, id int64, requesterID int64) error {
	v, err := s.videoRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if v.UserID != requesterID {
		return errs.ErrVideoNotOwned
	}
	if err := s.videoRepo.Delete(ctx, id); err != nil {
		return err
	}
	return s.channelRepo.DecrVideosCount(ctx, v.ChannelID)
}
func (s *VideoService) ListByChannel(ctx context.Context, channelID int64, viewerID int64, limit, offset int) ([]*domainvideo.Video, int, error) {
	return s.videoRepo.ListByChannel(ctx, channelID, viewerID, limit, offset)
}
func (s *VideoService) Feed(ctx context.Context, subscriberID int64, limit, offset int) ([]*domainvideo.Video, int, error) {
	return s.videoRepo.Feed(ctx, subscriberID, limit, offset)
}
func (s *VideoService) Search(ctx context.Context, query, category string, limit, offset int) ([]*domainvideo.Video, int, error) {
	return s.videoRepo.Search(ctx, query, category, limit, offset)
}
func (s *VideoService) RecordView(ctx context.Context, videoID int64, userID *int64, ipHash string) error {
	return s.videoRepo.RecordView(ctx, videoID, userID, ipHash)
}
func (s *VideoService) LikeVideo(ctx context.Context, videoID, userID int64) error {
	return s.videoRepo.AddLike(ctx, userID, videoID)
}
func (s *VideoService) UnlikeVideo(ctx context.Context, videoID, userID int64) error {
	return s.videoRepo.RemoveLike(ctx, userID, videoID)
}

var _ domainvideo.Service = (*VideoService)(nil)
