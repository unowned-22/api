package service

import (
	"context"

	"github.com/unowned-22/api/internal/domain/event"
	domainvideo "github.com/unowned-22/api/internal/domain/video"
	"github.com/unowned-22/api/internal/domain/videocomment"
	"github.com/unowned-22/api/internal/errs"
)

type VideoCommentService struct {
	commentRepo videocomment.Repository
	videoRepo   domainvideo.Repository
	publisher   event.Publisher
}

func NewVideoCommentService(c videocomment.Repository, v domainvideo.Repository, p event.Publisher) *VideoCommentService {
	return &VideoCommentService{commentRepo: c, videoRepo: v, publisher: p}
}
func (s *VideoCommentService) AddComment(ctx context.Context, videoID, userID int64, parentID *int64, body string) (*videocomment.Comment, error) {
	if _, err := s.videoRepo.GetByID(ctx, videoID); err != nil {
		return nil, err
	}
	if parentID != nil {
		parent, err := s.commentRepo.GetByID(ctx, *parentID)
		if err != nil {
			return nil, err
		}
		if parent.ParentID != nil {
			return nil, errs.ErrVideoCommentNesting
		}
	}
	c := &videocomment.Comment{VideoID: videoID, UserID: userID, ParentID: parentID, Body: body}
	return c, s.commentRepo.Create(ctx, c)
}
func (s *VideoCommentService) DeleteComment(ctx context.Context, id int64, requesterID int64) error {
	c, err := s.commentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if c.UserID != requesterID {
		return errs.ErrVideoCommentNotOwned
	}
	return s.commentRepo.SoftDelete(ctx, id)
}
func (s *VideoCommentService) ListComments(ctx context.Context, videoID int64, viewerID int64, limit, offset int) ([]*videocomment.Comment, int, error) {
	return s.commentRepo.ListByVideo(ctx, videoID, limit, offset)
}
func (s *VideoCommentService) ListReplies(ctx context.Context, parentID int64, viewerID int64) ([]*videocomment.Comment, error) {
	return s.commentRepo.ListReplies(ctx, parentID)
}
func (s *VideoCommentService) LikeComment(ctx context.Context, commentID, userID int64) error {
	return s.commentRepo.AddLike(ctx, userID, commentID)
}
func (s *VideoCommentService) UnlikeComment(ctx context.Context, commentID, userID int64) error {
	return s.commentRepo.RemoveLike(ctx, userID, commentID)
}

var _ videocomment.Service = (*VideoCommentService)(nil)
