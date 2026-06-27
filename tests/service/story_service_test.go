package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/domain/story"
	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/pagination"
	service2 "github.com/unowned-22/api/internal/service"
)

type mockRepo struct {
	createCalled  bool
	lastCreate    *story.Story
	listReturn    []*story.Story
	getByIDReturn *story.Story
}

func (m *mockRepo) Create(ctx context.Context, s *story.Story) error {
	m.createCalled = true
	m.lastCreate = s
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
	if m.getByIDReturn != nil {
		return m.getByIDReturn, nil
	}
	return nil, errs.ErrStoryNotFound
}
func (m *mockRepo) Delete(ctx context.Context, id int64) error { return nil }

// Additional methods required by the StoryRepository interface
func (m *mockRepo) AddLike(ctx context.Context, viewerID int64, storyID int64) error    { return nil }
func (m *mockRepo) RemoveLike(ctx context.Context, viewerID int64, storyID int64) error { return nil }
func (m *mockRepo) AddReply(ctx context.Context, viewerID int64, storyID int64, message string) error {
	return nil
}
func (m *mockRepo) ListReplies(ctx context.Context, viewerID int64, storyID int64) ([]*story.Reply, error) {
	return nil, nil
}
func (m *mockRepo) ListExpired(ctx context.Context) ([]*story.Story, error) { return nil, nil }

// IsCloseFriend for tests: default false unless set by test via listReturnOwnerClose map (not needed here)
func (m *mockRepo) IsCloseFriend(ctx context.Context, ownerID, friendID int64) (bool, error) {
	// simple rule used by tests: if ownerID==30 return true for any friendID==1
	if ownerID == 30 && friendID == 1 {
		return true, nil
	}
	return false, nil
}

type mockFriendshipSvc struct {
	viewerID int64
	friends  map[int64]bool
}

func (m *mockFriendshipSvc) SendRequest(ctx context.Context, requesterID, addresseeID int64) (*friendship.Friendship, error) {
	return nil, nil
}
func (m *mockFriendshipSvc) Accept(ctx context.Context, userID int64, friendshipID int64) (*friendship.Friendship, error) {
	return nil, nil
}
func (m *mockFriendshipSvc) Reject(ctx context.Context, userID int64, friendshipID int64) (*friendship.Friendship, error) {
	return nil, nil
}
func (m *mockFriendshipSvc) Cancel(ctx context.Context, userID int64, friendshipID int64) error {
	return nil
}
func (m *mockFriendshipSvc) Remove(ctx context.Context, userID int64, friendshipID int64) error {
	return nil
}
func (m *mockFriendshipSvc) ListFriends(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Friendship, int64, error) {
	return nil, 0, nil
}
func (m *mockFriendshipSvc) ListIncomingRequests(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Friendship, int64, error) {
	return nil, 0, nil
}
func (m *mockFriendshipSvc) ListOutgoingRequests(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Friendship, int64, error) {
	return nil, 0, nil
}
func (m *mockFriendshipSvc) ListSuggestions(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Suggestion, int64, error) {
	return nil, 0, nil
}
func (m *mockFriendshipSvc) IsFriend(ctx context.Context, userA, userB int64) (bool, error) {
	// return true when map has entry for userB (author) and userA == viewer (1)
	if userA == 1 {
		return m.friends[userB], nil
	}
	return false, nil
}

func TestPublish_AppendsWithinLimit(t *testing.T) {
	m := &mockRepo{}
	svc := service2.NewStoryService(m, nil, nil)

	// one existing slide
	existingSlides, _ := json.Marshal([]map[string]any{{"id": "s1"}})
	m.listReturn = []*story.Story{{ID: 1, UserID: 42, Slides: existingSlides, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(1 * time.Hour)}}

	newSlides, _ := json.Marshal([]map[string]any{{"id": "s2"}})
	st, err := svc.Publish(context.Background(), 42, newSlides, string(story.VisibilityEveryone), 24, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !m.createCalled {
		t.Fatalf("expected Create to be called")
	}
	if st.UserID != 42 {
		t.Fatalf("unexpected user id: %d", st.UserID)
	}
}

func TestPublish_ExceedsLimit(t *testing.T) {
	m := &mockRepo{}
	svc := service2.NewStoryService(m, nil, nil)

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
	if m.createCalled {
		t.Fatalf("Create should not be called when limit exceeded")
	}
}

func TestPublish_InvalidVisibility(t *testing.T) {
	m := &mockRepo{}
	svc := service2.NewStoryService(m, nil, nil)

	slides, _ := json.Marshal([]map[string]any{{"id": "x"}})
	_, err := svc.Publish(context.Background(), 1, slides, "invalid-vis", 24, nil)
	if err == nil {
		t.Fatalf("expected error for invalid visibility")
	}
	if err != errs.ErrInvalidStoryPayload {
		t.Fatalf("expected ErrInvalidStoryPayload, got: %v", err)
	}
}

func TestListVisibleStories_CoversVisibilities(t *testing.T) {
	// viewer is user 1
	viewer := int64(1)

	// setup stories per author
	now := time.Now()
	sEveryone := &story.Story{ID: 11, UserID: 10, Visibility: story.VisibilityEveryone, Slides: []byte("[]"), CreatedAt: now, ExpiresAt: now.Add(1 * time.Hour)}
	sFriends := &story.Story{ID: 12, UserID: 20, Visibility: story.VisibilityFriends, Slides: []byte("[]"), CreatedAt: now, ExpiresAt: now.Add(1 * time.Hour)}
	sClose := &story.Story{ID: 13, UserID: 30, Visibility: story.VisibilityClose, Slides: []byte("[]"), CreatedAt: now, ExpiresAt: now.Add(1 * time.Hour)}

	// mock friendship service: viewer is friend with author 20
	mf := &mockFriendshipSvc{friends: map[int64]bool{20: true}}
	m := &mockRepo{}
	svc := service2.NewStoryService(m, mf, nil)

	// Everyone should see author's stories
	m.listReturn = []*story.Story{sEveryone}
	out, err := svc.ListVisibleStories(context.Background(), viewer, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 story for everyone visibility, got %d", len(out))
	}

	// Friends: should see when friendship exists
	m.listReturn = []*story.Story{sFriends}
	out, err = svc.ListVisibleStories(context.Background(), viewer, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 story for friends visibility when friend, got %d", len(out))
	}

	// Close: mockRepo.IsCloseFriend returns true for owner 30 and friend 1
	m.listReturn = []*story.Story{sClose}
	out, err = svc.ListVisibleStories(context.Background(), viewer, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 story for close visibility when close friend, got %d", len(out))
	}
}

func TestAccessControl_DeniesUnauthorized(t *testing.T) {
	// viewer is user 2 (not friend)
	viewer := int64(2)

	// story owned by author 10 with friends visibility
	now := time.Now()
	sFriends := &story.Story{ID: 101, UserID: 10, Visibility: story.VisibilityFriends, Slides: []byte("[]"), CreatedAt: now, ExpiresAt: now.Add(1 * time.Hour)}

	mf := &mockFriendshipSvc{friends: map[int64]bool{}}
	m := &mockRepo{getByIDReturn: sFriends}
	svc := service2.NewStoryService(m, mf, nil)

	if err := svc.Like(context.Background(), viewer, sFriends.ID); err != errs.ErrForbidden {
		t.Fatalf("expected ErrForbidden for liking friends-only story by non-friend, got: %v", err)
	}
	if err := svc.Reply(context.Background(), viewer, sFriends.ID, "hi"); err != errs.ErrForbidden {
		t.Fatalf("expected ErrForbidden for replying to friends-only story by non-friend, got: %v", err)
	}
	if err := svc.AddView(context.Background(), viewer, sFriends.ID, nil); err != errs.ErrForbidden {
		t.Fatalf("expected ErrForbidden for viewing friends-only story by non-friend, got: %v", err)
	}

	// close visibility: owner 30 has IsCloseFriend true only for friend 1 per mock.
	sClose := &story.Story{ID: 202, UserID: 30, Visibility: story.VisibilityClose, Slides: []byte("[]"), CreatedAt: now, ExpiresAt: now.Add(1 * time.Hour)}
	m2 := &mockRepo{getByIDReturn: sClose}
	svc2 := service2.NewStoryService(m2, mf, nil)
	// viewer 2 is not close friend
	if err := svc2.Like(context.Background(), viewer, sClose.ID); err != errs.ErrForbidden {
		t.Fatalf("expected ErrForbidden for liking close-only story by non-close friend, got: %v", err)
	}
}
