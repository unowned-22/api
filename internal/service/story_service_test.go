package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/unowned-22/api/internal/domain/story"
	"github.com/unowned-22/api/internal/errs"
)

type mockRepo struct {
	upsertCalled bool
	lastUpsert   *story.Story
	listReturn   []*story.Story
}

func (m *mockRepo) Upsert(ctx context.Context, s *story.Story) error {
	m.upsertCalled = true
	m.lastUpsert = s
	return nil
}
func (m *mockRepo) ListActiveByUser(ctx context.Context, userID int64) ([]*story.Story, error) {
	return m.listReturn, nil
}
func (m *mockRepo) ListFeed(ctx context.Context, viewerID int64) ([]*story.Story, error) {
	return nil, nil
}
func (m *mockRepo) AddView(ctx context.Context, viewerID int64, storyID int64, slideIndex *int) error {
	return nil
}
func (m *mockRepo) ListViewsByViewer(ctx context.Context, viewerID int64) (map[int64]map[int]bool, error) {
	return nil, nil
}
func (m *mockRepo) GetByID(ctx context.Context, id int64) (*story.Story, error) {
	return nil, errs.ErrStoryNotFound
}
func (m *mockRepo) Delete(ctx context.Context, id int64) error { return nil }

// Additional methods required by the StoryRepository interface
func (m *mockRepo) AddLike(ctx context.Context, viewerID int64, storyID int64) error    { return nil }
func (m *mockRepo) RemoveLike(ctx context.Context, viewerID int64, storyID int64) error { return nil }
func (m *mockRepo) AddReply(ctx context.Context, viewerID int64, storyID int64, message string) error {
	return nil
}
func (m *mockRepo) ListReplies(ctx context.Context, storyID int64) ([]*story.Reply, error) {
	return nil, nil
}
func (m *mockRepo) ListExpired(ctx context.Context) ([]*story.Story, error) { return nil, nil }

func TestPublish_AppendsWithinLimit(t *testing.T) {
	m := &mockRepo{}
	svc := NewStoryService(m, nil)

	// one existing slide
	existingSlides, _ := json.Marshal([]map[string]any{{"id": "s1"}})
	m.listReturn = []*story.Story{{ID: 1, UserID: 42, Slides: existingSlides, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(1 * time.Hour)}}

	newSlides, _ := json.Marshal([]map[string]any{{"id": "s2"}})
	st, err := svc.Publish(context.Background(), 42, newSlides, string(story.VisibilityEveryone), 24, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !m.upsertCalled {
		t.Fatalf("expected Upsert to be called")
	}
	if st.UserID != 42 {
		t.Fatalf("unexpected user id: %d", st.UserID)
	}
}

func TestPublish_ExceedsLimit(t *testing.T) {
	m := &mockRepo{}
	svc := NewStoryService(m, nil)

	// existing 19 slides
	ex := make([]map[string]any, 19)
	for i := range ex {
		ex[i] = map[string]any{"id": i}
	}
	existingSlides, _ := json.Marshal(ex)
	m.listReturn = []*story.Story{{ID: 1, UserID: 7, Slides: existingSlides, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(1 * time.Hour)}}

	// publish 2 slides -> exceeds 20
	newSlides, _ := json.Marshal([]map[string]any{{"id": "a"}, {"id": "b"}})
	_, err := svc.Publish(context.Background(), 7, newSlides, string(story.VisibilityEveryone), 24, nil)
	if err == nil {
		t.Fatalf("expected error due to exceeding slide limit")
	}
	if err != errs.ErrInvalidStoryPayload {
		t.Fatalf("expected ErrInvalidStoryPayload, got: %v", err)
	}
	if m.upsertCalled {
		t.Fatalf("Upsert should not be called when limit exceeded")
	}
}

func TestPublish_InvalidVisibility(t *testing.T) {
	m := &mockRepo{}
	svc := NewStoryService(m, nil)

	slides, _ := json.Marshal([]map[string]any{{"id": "x"}})
	_, err := svc.Publish(context.Background(), 1, slides, "invalid-vis", 24, nil)
	if err == nil {
		t.Fatalf("expected error for invalid visibility")
	}
	if err != errs.ErrInvalidStoryPayload {
		t.Fatalf("expected ErrInvalidStoryPayload, got: %v", err)
	}
}
