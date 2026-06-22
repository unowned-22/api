package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/domain/story"
	"github.com/unowned-22/api/internal/errs"
)

type StoryService struct {
	repo          story.StoryRepository
	friendshipSvc friendship.Service
}

func NewStoryService(repo story.StoryRepository, friendshipSvc friendship.Service) *StoryService {
	return &StoryService{repo: repo, friendshipSvc: friendshipSvc}
}

func (s *StoryService) Publish(ctx context.Context, userID int64, slidesJSON []byte, visibility string, durationHours int, hiddenFrom []int64) (*story.Story, error) {
	// validate visibility
	switch visibility {
	case string(story.VisibilityEveryone), string(story.VisibilityFriends), string(story.VisibilityClose):
	default:
		return nil, errs.ErrInvalidStoryPayload
	}

	// validate duration
	switch durationHours {
	case 1, 12, 24, 48:
	default:
		return nil, errs.ErrInvalidStoryPayload
	}

	// validate slides JSON -- shallow checks
	var slides []json.RawMessage
	if err := json.Unmarshal(slidesJSON, &slides); err != nil {
		return nil, errs.ErrInvalidStoryPayload
	}
	if len(slides) == 0 || len(slides) > 20 {
		return nil, errs.ErrInvalidStoryPayload
	}

	// inspect each slide for blob: media URLs
	for _, sraw := range slides {
		var m map[string]any
		if err := json.Unmarshal(sraw, &m); err != nil {
			return nil, errs.ErrInvalidStoryPayload
		}
		bgVal, ok := m["background"]
		if !ok || bgVal == nil {
			continue
		}
		if bg, ok := bgVal.(map[string]any); ok {
			if kind, ok := bg["kind"].(string); ok && kind == "media" {
				if urlv, ok := bg["url"].(string); ok && len(urlv) >= 5 && urlv[:5] == "blob:" {
					return nil, errs.ErrInvalidStoryPayload
				}
			}
		}
	}

	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(durationHours) * time.Hour)

	st := &story.Story{
		UserID:            userID,
		Visibility:        story.Visibility(visibility),
		DurationHours:     durationHours,
		HiddenFromUserIDs: hiddenFrom,
		Slides:            slidesJSON,
		CreatedAt:         now,
		ExpiresAt:         expiresAt,
	}

	if err := s.repo.Create(ctx, st); err != nil {
		return nil, fmt.Errorf("failed to persist story: %w", err)
	}

	return st, nil
}

func (s *StoryService) ListMyStories(ctx context.Context, userID int64) ([]*story.Story, error) {
	return s.repo.ListActiveByUser(ctx, userID)
}

// ListVisibleStories returns stories of an author visible to viewer according to visibility rules.
func (s *StoryService) ListVisibleStories(ctx context.Context, viewerID, authorID int64) ([]*story.Story, error) {
	sts, err := s.repo.ListActiveByUser(ctx, authorID)
	if err != nil {
		return nil, err
	}
	out := make([]*story.Story, 0)
	for _, st := range sts {
		switch st.Visibility {
		case story.VisibilityEveryone:
			out = append(out, st)
		case story.VisibilityFriends:
			isFriend, ferr := s.friendshipSvc.IsFriend(ctx, viewerID, authorID)
			if ferr != nil {
				return nil, ferr
			}
			if isFriend {
				out = append(out, st)
			}
		case story.VisibilityClose:
			// TODO: close-friends list not implemented in this task.
		}
	}
	return out, nil
}

func (s *StoryService) Delete(ctx context.Context, userID int64, storyID int64) error {
	st, err := s.repo.GetByID(ctx, storyID)
	if err != nil {
		return err
	}
	if st.UserID != userID {
		return errs.ErrForbidden
	}
	return s.repo.Delete(ctx, storyID)
}
