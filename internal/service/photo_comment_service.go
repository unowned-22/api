package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/photo"
	"github.com/unowned-22/api/internal/domain/photocomment"
	"github.com/unowned-22/api/internal/errs"
)

type photoCommentService struct {
	repo      photocomment.Repository
	photos    photo.Repository
	publisher event.Publisher
}

func NewPhotoCommentService(repo photocomment.Repository, photos photo.Repository, publisher event.Publisher) photocomment.Service {
	return &photoCommentService{repo: repo, photos: photos, publisher: publisher}
}

func (s *photoCommentService) AddComment(ctx context.Context, photoID int64, authorID int64, input photocomment.AddCommentInput) (*photocomment.Comment, error) {
	// check photo exists and basic visibility
	p, err := s.photos.GetByID(ctx, photoID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errs.ErrPhotoNotFound
	}
	// visibility: owner allowed
	if p.UserID != authorID {
		if p.Visibility == photo.VisibilityNobody {
			return nil, errs.ErrPhotoNotFound
		}
		for _, hid := range p.HiddenFrom {
			if hid == authorID {
				return nil, errs.ErrPhotoNotFound
			}
		}
		if p.Visibility != photo.VisibilityEveryone {
			return nil, errs.ErrPhotoNotFound
		}
	}

	// if parent provided, validate
	if input.ParentID != nil {
		parent, err := s.repo.GetByID(ctx, *input.ParentID)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			return nil, errs.ErrCommentNotFound
		}
		if parent.PhotoID != photoID {
			return nil, errs.ErrCommentNestingNotAllowed
		}
		if parent.ParentID != nil {
			return nil, errs.ErrCommentNestingNotAllowed
		}
	}

	c := &photocomment.Comment{
		PhotoID:   photoID,
		AuthorID:  authorID,
		ParentID:  input.ParentID,
		Body:      input.Body,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	// load full comment
	created, err := s.repo.GetByID(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	if p.UserID != authorID {
		if s.publisher != nil {
			payload, _ := json.Marshal(map[string]any{"photo_id": photoID, "comment_id": created.ID, "owner_id": p.UserID, "actor_id": authorID})
			if err := s.publisher.Publish(ctx, event.Event{Name: event.PhotoCommented, Payload: payload}); err != nil {
				return nil, err
			}
		}
	}
	if created.ParentID != nil {
		parent, _ := s.repo.GetByID(ctx, *created.ParentID)
		if parent != nil && parent.AuthorID != authorID {
			if s.publisher != nil {
				payload, _ := json.Marshal(map[string]any{"comment_id": created.ID, "parent_comment_id": parent.ID, "owner_id": parent.AuthorID, "actor_id": authorID})
				if err := s.publisher.Publish(ctx, event.Event{Name: event.CommentReplied, Payload: payload}); err != nil {
					return nil, err
				}
			}
		}
	}

	return created, nil
}

func (s *photoCommentService) GetComment(ctx context.Context, id int64, viewerID int64) (*photocomment.Comment, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *photoCommentService) ListComments(ctx context.Context, photoID int64, viewerID int64, limit, offset int) ([]*photocomment.Comment, int, error) {
	return s.repo.ListRoots(ctx, photoID, viewerID, limit, offset)
}

func (s *photoCommentService) ListReplies(ctx context.Context, parentID int64, viewerID int64, limit, offset int) ([]*photocomment.Comment, int, error) {
	return s.repo.ListReplies(ctx, parentID, viewerID, limit, offset)
}

func (s *photoCommentService) EditComment(ctx context.Context, id int64, requesterID int64, body string) (*photocomment.Comment, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, errs.ErrCommentNotFound
	}
	if c.AuthorID != requesterID {
		return nil, errs.ErrCommentNotOwned
	}
	if time.Since(c.CreatedAt) > 15*time.Minute {
		return nil, errs.ErrCommentEditExpired
	}
	if err := s.repo.Update(ctx, id, body); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *photoCommentService) DeleteComment(ctx context.Context, id int64, requesterID int64) error {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if c == nil {
		return errs.ErrCommentNotFound
	}
	if c.AuthorID != requesterID {
		return errs.ErrCommentNotOwned
	}
	if c.IsDeleted {
		return errs.ErrCommentAlreadyDeleted
	}
	return s.repo.SoftDelete(ctx, id)
}

func (s *photoCommentService) LikeComment(ctx context.Context, commentID int64, userID int64) error {
	c, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return err
	}
	if c == nil {
		return errs.ErrCommentNotFound
	}
	if err := s.repo.AddLike(ctx, userID, commentID); err != nil {
		return err
	}
	if c.AuthorID != userID {
		if s.publisher != nil {
			payload, _ := json.Marshal(map[string]any{"comment_id": commentID, "owner_id": c.AuthorID, "actor_id": userID})
			if err := s.publisher.Publish(ctx, event.Event{Name: event.CommentLiked, Payload: payload}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *photoCommentService) UnlikeComment(ctx context.Context, commentID int64, userID int64) error {
	if err := s.repo.RemoveLike(ctx, userID, commentID); err != nil {
		return err
	}
	return nil
}
