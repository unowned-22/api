package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/domain/photo"
	"github.com/unowned-22/api/internal/domain/photocomment"
	"github.com/unowned-22/api/internal/errs"
)

type photoCommentService struct {
	repo      photocomment.Repository
	photos    photo.Repository
	notifRepo notification.Repository
}

func NewPhotoCommentService(repo photocomment.Repository, photos photo.Repository, notifRepo notification.Repository) photocomment.Service {
	return &photoCommentService{repo: repo, photos: photos, notifRepo: notifRepo}
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

	// notifications (fire-and-forget but try best-effort)
	go func() {
		// notify photo owner if different
		if p.UserID != authorID {
			payload := map[string]interface{}{"photo_id": photoID, "comment_id": created.ID}
			b, _ := json.Marshal(payload)
			_ = s.notifRepo.Create(context.Background(), &notification.Notification{
				UserID:     p.UserID,
				ActorID:    authorID,
				Type:       notification.Type("photo_commented"),
				EntityType: "photo",
				EntityID:   photoID,
				Payload:    b,
			})
		}
		// if reply to parent, notify parent author
		if created.ParentID != nil {
			parent, _ := s.repo.GetByID(context.Background(), *created.ParentID)
			if parent != nil && parent.AuthorID != authorID {
				payload := map[string]interface{}{"comment_id": created.ID}
				b, _ := json.Marshal(payload)
				_ = s.notifRepo.Create(context.Background(), &notification.Notification{
					UserID:     parent.AuthorID,
					ActorID:    authorID,
					Type:       notification.Type("comment_replied"),
					EntityType: "photo_comment",
					EntityID:   parent.ID,
					Payload:    b,
				})
			}
		}
	}()

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
	// notify comment author
	if c.AuthorID != userID {
		payload := map[string]interface{}{"comment_id": commentID}
		b, _ := json.Marshal(payload)
		go s.notifRepo.Create(context.Background(), &notification.Notification{
			UserID:     c.AuthorID,
			ActorID:    userID,
			Type:       notification.Type("comment_liked"),
			EntityType: "photo_comment",
			EntityID:   c.ID,
			Payload:    b,
		})
	}
	return nil
}

func (s *photoCommentService) UnlikeComment(ctx context.Context, commentID int64, userID int64) error {
	if err := s.repo.RemoveLike(ctx, userID, commentID); err != nil {
		return err
	}
	return nil
}
