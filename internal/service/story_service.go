package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/unowned-22/api/internal/domain/story"
	"github.com/unowned-22/api/internal/errs"
)

type StoryService struct {
	repo story.StoryRepository
}

func NewStoryService(repo story.StoryRepository) *StoryService {
	return &StoryService{repo: repo}
}

func (s *StoryService) Feed(ctx context.Context, userID int64) ([]*story.Story, error) {
	return s.repo.ListFeed(ctx, userID)
}

func (s *StoryService) AddView(ctx context.Context, viewerID int64, storyID int64, slideIndex *int) error {
	return s.repo.AddView(ctx, viewerID, storyID, slideIndex)
}

func (s *StoryService) ListViewsByViewer(ctx context.Context, viewerID int64) (map[int64]map[int]bool, error) {
	return s.repo.ListViewsByViewer(ctx, viewerID)
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

	// Enforce global per-user slide limit (20) by inspecting existing story
	existing, err := s.repo.ListActiveByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing stories: %w", err)
	}
	existingCount := 0
	if len(existing) > 0 {
		var exSlides []json.RawMessage
		if err := json.Unmarshal(existing[0].Slides, &exSlides); err == nil {
			existingCount = len(exSlides)
		}
	}
	if existingCount+len(slides) > 20 {
		return nil, errs.ErrInvalidStoryPayload
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

	if err := s.repo.Upsert(ctx, st); err != nil {
		return nil, fmt.Errorf("failed to persist story: %w", err)
	}

	return st, nil
}

func (s *StoryService) ListMyStories(ctx context.Context, userID int64) ([]*story.Story, error) {
	return s.repo.ListActiveByUser(ctx, userID)
}

func (s *StoryService) Like(ctx context.Context, viewerID int64, storyID int64) error {
	return s.repo.AddLike(ctx, viewerID, storyID)
}

func (s *StoryService) Unlike(ctx context.Context, viewerID int64, storyID int64) error {
	return s.repo.RemoveLike(ctx, viewerID, storyID)
}

func (s *StoryService) Reply(ctx context.Context, viewerID int64, storyID int64, message string) error {
	if message == "" {
		return errs.ErrInvalidStoryPayload
	}
	return s.repo.AddReply(ctx, viewerID, storyID, message)
}

func (s *StoryService) ListReplies(ctx context.Context, storyID int64) ([]*story.Reply, error) {
	return s.repo.ListReplies(ctx, storyID)
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
