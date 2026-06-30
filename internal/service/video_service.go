package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/unowned-22/api/internal/domain/community"
	"github.com/unowned-22/api/internal/domain/communitypost"
	"github.com/unowned-22/api/internal/domain/event"
	mediavideo "github.com/unowned-22/api/internal/domain/media/video"
	"github.com/unowned-22/api/internal/domain/storage"
	domainvideo "github.com/unowned-22/api/internal/domain/video"
	"github.com/unowned-22/api/internal/errs"
)

type VideoService struct {
	videoRepo         domainvideo.Repository
	communitySvc      community.Service
	communityPostRepo communitypost.Repository
	storage           storage.Storage
	jobQueue          domainvideo.JobQueue
	bucket            string
	publisher         event.Publisher
	maxFileSize       int64
}

func NewVideoService(
	v domainvideo.Repository,
	c community.Service,
	cp communitypost.Repository,
	s storage.Storage,
	q domainvideo.JobQueue,
	bucket string,
	pub event.Publisher,
	maxFileSize int64,
) *VideoService {
	return &VideoService{
		videoRepo:         v,
		communitySvc:      c,
		communityPostRepo: cp,
		storage:           s,
		jobQueue:          q,
		bucket:            bucket,
		publisher:         pub,
		maxFileSize:       maxFileSize,
	}
}

func (s *VideoService) Upload(ctx context.Context, req domainvideo.UploadRequest) (*domainvideo.Video, error) {
	if req.SizeBytes > s.maxFileSize {
		return nil, errs.ErrAttachmentTooLarge
	}
	if req.ContentType != "video/mp4" && req.ContentType != "video/webm" && req.ContentType != "video/quicktime" {
		return nil, errs.ErrUnsupportedVideoType
	}

	if err := s.communitySvc.RequireAdminOrOwner(ctx, req.CommunityID, req.UserID); err != nil {
		return nil, err
	}

	key := path.Join("videos", fmt.Sprintf("%d", req.CommunityID), "raw", uuid.NewString(), req.FileName)
	if _, err := s.storage.PutObject(ctx, s.bucket, key, req.Body.(io.Reader), req.SizeBytes, req.ContentType); err != nil {
		return nil, err
	}

	v := &domainvideo.Video{
		UserID:      req.UserID,
		CommunityID: req.CommunityID,
		Title:       req.Title,
		Description: req.Description,
		Category:    req.Category,
		Tags:        req.Tags,
		Visibility:  req.Visibility,
		Status:      domainvideo.StatusPending,
		RawKey:      key,
		SizeBytes:   req.SizeBytes,
	}
	if err := s.videoRepo.Create(ctx, v); err != nil {
		return nil, err
	}
	if err := s.videoRepo.SetTags(ctx, v.ID, req.Tags); err != nil {
		return nil, err
	}

	_ = s.communitySvc.IncrVideosCount(ctx, req.CommunityID)

	_ = s.jobQueue.Enqueue(ctx, mediavideo.ProcessJob{
		UserID:      req.UserID,
		VideoID:     v.ID,
		CommunityID: req.CommunityID,
		RawKey:      key,
	})
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
	if v.PublishedAt == nil && viewerID != v.UserID {
		err := s.communitySvc.RequireAdminOrOwner(ctx, v.CommunityID, viewerID)
		if err != nil {
			return nil, errs.ErrVideoNotFound
		}
	}
	if viewerID != 0 {
		if liked, err := s.videoRepo.IsLiked(ctx, viewerID, id); err == nil {
			v.IsLiked = liked
		}
	}
	return v, nil
}

// ── Update ───────────────────────────────────────────────────────────────────

func (s *VideoService) UpdateMeta(ctx context.Context, id int64, requesterID int64, req domainvideo.UpdateMetaRequest) (*domainvideo.Video, error) {
	v, err := s.videoRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Allow owner of the video OR community admin/owner.
	if v.UserID != requesterID {
		if authErr := s.communitySvc.RequireAdminOrOwner(ctx, v.CommunityID, requesterID); authErr != nil {
			return nil, errs.ErrVideoNotOwned
		}
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

// ── Delete ───────────────────────────────────────────────────────────────────

func (s *VideoService) Delete(ctx context.Context, id int64, requesterID int64) error {
	v, err := s.videoRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if v.UserID != requesterID {
		if authErr := s.communitySvc.RequireAdminOrOwner(ctx, v.CommunityID, requesterID); authErr != nil {
			return errs.ErrVideoNotOwned
		}
	}
	return s.videoRepo.Delete(ctx, id)
}

// ── Publish / Unpublish ──────────────────────────────────────────────────────

func (s *VideoService) Publish(ctx context.Context, videoID, requesterID int64, targets []string) error {
	v, err := s.videoRepo.GetByID(ctx, videoID)
	if err != nil {
		return err
	}
	if v.Status != domainvideo.StatusReady {
		return fmt.Errorf("video is not ready for publishing (status=%s)", v.Status)
	}
	if err := s.communitySvc.RequireAdminOrOwner(ctx, v.CommunityID, requesterID); err != nil {
		return err
	}
	if len(targets) == 0 {
		targets = []string{domainvideo.PublishTargetVideoFeed}
	}
	if err := s.videoRepo.Publish(ctx, videoID, targets); err != nil {
		return err
	}

	for _, t := range targets {
		if t == domainvideo.PublishTargetVideoFeed {
			if post, postErr := s.communityPostRepo.CreateForVideo(ctx, v.CommunityID, requesterID, videoID); postErr == nil {
				_ = s.communitySvc.IncrPostsCount(ctx, v.CommunityID)
				bridgePayload, _ := json.Marshal(map[string]any{
					"community_id": v.CommunityID,
					"post_id":      post.ID,
					"text":         v.Title,
				})
				_ = s.publisher.Publish(ctx, event.Event{Name: event.CommunityPostPublished, Payload: bridgePayload})
			}

			break
		}
	}

	// Publish event for the realtime pipeline.
	comm, err := s.communitySvc.GetByID(ctx, v.CommunityID)
	if err != nil {
		return nil // best-effort; video is already published
	}
	payload, _ := json.Marshal(map[string]any{
		"video_id":      videoID,
		"community_id":  v.CommunityID,
		"channel_name":  comm.Name, // kept as channel_name for WS payload compatibility
		"title":         v.Title,
		"thumbnail_key": v.ThumbnailKey,
	})
	_ = s.publisher.Publish(ctx, event.Event{Name: event.VideoPublished, Payload: payload})
	return nil
}

func (s *VideoService) Unpublish(ctx context.Context, videoID, requesterID int64) error {
	v, err := s.videoRepo.GetByID(ctx, videoID)
	if err != nil {
		return err
	}
	if err := s.communitySvc.RequireAdminOrOwner(ctx, v.CommunityID, requesterID); err != nil {
		return err
	}
	if err := s.videoRepo.Unpublish(ctx, videoID); err != nil {
		return err
	}

	if communityID, postErr := s.communityPostRepo.SoftDeleteByVideoID(ctx, videoID); postErr == nil && communityID != nil {
		_ = s.communitySvc.DecrPostsCount(ctx, *communityID)
	}
	return nil
}

// ── Boost stub ───────────────────────────────────────────────────────────────

func (s *VideoService) Boost(ctx context.Context, videoID, requesterID int64, hours int) error {
	// TODO: billing / ranking logic is explicitly out of scope.
	// See TASK-communities §4, §10 and AGENTS.md "Boost / promotion stub".
	v, err := s.videoRepo.GetByID(ctx, videoID)
	if err != nil {
		return err
	}
	if err := s.communitySvc.RequireAdminOrOwner(ctx, v.CommunityID, requesterID); err != nil {
		return err
	}
	if hours <= 0 || hours > 720 {
		return fmt.Errorf("hours must be between 1 and 720")
	}
	until := time.Now().UTC().Add(time.Duration(hours) * time.Hour).Format(time.RFC3339)
	return s.videoRepo.SetBoostedUntil(ctx, videoID, &until)
}

// ── Listings ─────────────────────────────────────────────────────────────────

func (s *VideoService) ListByCommunity(ctx context.Context, communityID int64, viewerID int64, limit, offset int) ([]*domainvideo.Video, int, error) {
	return s.videoRepo.ListByCommunity(ctx, communityID, viewerID, limit, offset)
}

func (s *VideoService) ListDrafts(ctx context.Context, communityID, requesterID int64, limit, offset int) ([]*domainvideo.Video, int, error) {
	if err := s.communitySvc.RequireAdminOrOwner(ctx, communityID, requesterID); err != nil {
		return nil, 0, err
	}
	return s.videoRepo.ListDraftsByCommunity(ctx, communityID, limit, offset)
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

// ── Compile-time check ───────────────────────────────────────────────────────

var _ domainvideo.Service = (*VideoService)(nil)
