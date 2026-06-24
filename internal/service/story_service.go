package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/domain/story"
	"github.com/unowned-22/api/internal/errs"
)

type StoryService struct {
	repo          story.StoryRepository
	friendshipSvc friendship.Service
	publisher     event.Publisher
}

func NewStoryService(repo story.StoryRepository, friendshipSvc friendship.Service, publisher event.Publisher) *StoryService {
	return &StoryService{repo: repo, friendshipSvc: friendshipSvc, publisher: publisher}
}

func (s *StoryService) Feed(ctx context.Context, userID int64) ([]*story.Story, error) {
	return s.repo.ListFeed(ctx, userID)
}

func (s *StoryService) AddView(ctx context.Context, viewerID int64, storyID int64, slideIndex *int) error {
	if _, err := s.assertCanAccess(ctx, viewerID, storyID); err != nil {
		return err
	}
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

	// best-effort publish story.published event
	if s.publisher != nil {
		payload, _ := json.Marshal(map[string]interface{}{"story_id": st.ID, "user_id": st.UserID, "visibility": st.Visibility, "hidden_from": st.HiddenFromUserIDs})
		if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.StoryPublished, Payload: payload}); pubErr != nil {
			fmt.Printf("failed to publish story.published: %v\n", pubErr)
		}
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
			isClose, cerr := s.repo.IsCloseFriend(ctx, authorID, viewerID)
			if cerr != nil {
				return nil, cerr
			}
			if isClose {
				out = append(out, st)
			}
		}
	}
	return out, nil
}

func (s *StoryService) Like(ctx context.Context, viewerID int64, storyID int64) error {
	if _, err := s.assertCanAccess(ctx, viewerID, storyID); err != nil {
		return err
	}
	return s.repo.AddLike(ctx, viewerID, storyID)
}

func (s *StoryService) Unlike(ctx context.Context, viewerID int64, storyID int64) error {
	if _, err := s.assertCanAccess(ctx, viewerID, storyID); err != nil {
		return err
	}
	return s.repo.RemoveLike(ctx, viewerID, storyID)
}

func (s *StoryService) Reply(ctx context.Context, viewerID int64, storyID int64, message string) error {
	if message == "" {
		return errs.ErrInvalidStoryPayload
	}
	if len(message) > 500 {
		return errs.ErrInvalidStoryPayload
	}
	if _, err := s.assertCanAccess(ctx, viewerID, storyID); err != nil {
		return err
	}
	return s.repo.AddReply(ctx, viewerID, storyID, message)
}

func (s *StoryService) ListReplies(ctx context.Context, viewerID int64, storyID int64) ([]*story.Reply, error) {
	if _, err := s.assertCanAccess(ctx, viewerID, storyID); err != nil {
		return nil, err
	}
	return s.repo.ListReplies(ctx, viewerID, storyID)
}

// assertCanAccess checks whether viewerID can access storyID according to
// visibility rules and hidden-from list. Returns the story when allowed.
func (s *StoryService) assertCanAccess(ctx context.Context, viewerID int64, storyID int64) (*story.Story, error) {
	st, err := s.repo.GetByID(ctx, storyID)
	if err != nil {
		return nil, err
	}
	// treat expired as not found
	if time.Now().UTC().After(st.ExpiresAt) || st.ExpiresAt.Equal(time.Time{}) {
		return nil, errs.ErrStoryNotFound
	}
	// author always has access
	if viewerID == st.UserID {
		return st, nil
	}
	// hidden-from list prevents access
	for _, hid := range st.HiddenFromUserIDs {
		if hid == viewerID {
			return nil, errs.ErrForbidden
		}
	}
	switch st.Visibility {
	case story.VisibilityEveryone:
		return st, nil
	case story.VisibilityFriends:
		isFriend, ferr := s.friendshipSvc.IsFriend(ctx, viewerID, st.UserID)
		if ferr != nil {
			return nil, ferr
		}
		if isFriend {
			return st, nil
		}
		return nil, errs.ErrForbidden
	case story.VisibilityClose:
		isClose, cerr := s.repo.IsCloseFriend(ctx, st.UserID, viewerID)
		if cerr != nil {
			return nil, cerr
		}
		if isClose {
			return st, nil
		}
		return nil, errs.ErrForbidden
	default:
		return nil, errs.ErrForbidden
	}
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
