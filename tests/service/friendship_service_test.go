package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/pagination"
	service2 "github.com/unowned-22/api/internal/service"
)

// simple in-memory mock repository
type mockFriendRepo struct {
	data map[int64]*friendship.Friendship
	seq  int64
}

func newMockFriendRepo() *mockFriendRepo {
	return &mockFriendRepo{data: make(map[int64]*friendship.Friendship)}
}

func (m *mockFriendRepo) Create(ctx context.Context, requesterID, addresseeID int64) (*friendship.Friendship, error) {
	m.seq++
	now := time.Now()
	f := &friendship.Friendship{ID: m.seq, RequesterID: requesterID, AddresseeID: addresseeID, Status: friendship.StatusPending, CreatedAt: now, UpdatedAt: now}
	m.data[f.ID] = f
	return f, nil
}

func (m *mockFriendRepo) UpdateStatus(ctx context.Context, id int64, status friendship.Status) (*friendship.Friendship, error) {
	f, ok := m.data[id]
	if !ok {
		return nil, errs.ErrFriendshipNotFound
	}
	f.Status = status
	f.UpdatedAt = time.Now()
	return f, nil
}

func (m *mockFriendRepo) GetByUsers(ctx context.Context, userA, userB int64) (*friendship.Friendship, error) {
	for _, f := range m.data {
		if (f.RequesterID == userA && f.AddresseeID == userB) || (f.RequesterID == userB && f.AddresseeID == userA) {
			return f, nil
		}
	}
	return nil, nil
}

func (m *mockFriendRepo) GetByID(ctx context.Context, id int64) (*friendship.Friendship, error) {
	f, ok := m.data[id]
	if !ok {
		return nil, nil
	}
	return f, nil
}

func (m *mockFriendRepo) Delete(ctx context.Context, id int64) error {
	if _, ok := m.data[id]; !ok {
		return errs.ErrFriendshipNotFound
	}
	delete(m.data, id)
	return nil
}

// Remaining methods not used in tests — provide minimal implementations
func (m *mockFriendRepo) ListFriends(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Friendship, int64, error) {
	return nil, 0, nil
}
func (m *mockFriendRepo) ListIncomingRequests(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Friendship, int64, error) {
	return nil, 0, nil
}
func (m *mockFriendRepo) ListOutgoingRequests(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Friendship, int64, error) {
	return nil, 0, nil
}
func (m *mockFriendRepo) IsFriend(ctx context.Context, userA, userB int64) (bool, error) {
	for _, f := range m.data {
		if ((f.RequesterID == userA && f.AddresseeID == userB) || (f.RequesterID == userB && f.AddresseeID == userA)) && f.Status == friendship.StatusAccepted {
			return true, nil
		}
	}
	return false, nil
}
func (m *mockFriendRepo) IsSubscriber(ctx context.Context, requesterID, addresseeID int64) (bool, error) {
	return false, nil
}
func (m *mockFriendRepo) GetFriendIDs(ctx context.Context, userID int64) ([]int64, error) {
	return nil, nil
}

func (m *mockFriendRepo) CountFriends(ctx context.Context, userID int64) (int64, error) {
	var cnt int64
	for _, f := range m.data {
		if (f.RequesterID == userID || f.AddresseeID == userID) && f.Status == friendship.StatusAccepted {
			cnt++
		}
	}
	return cnt, nil
}

func (m *mockFriendRepo) ListSuggestions(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Suggestion, int64, error) {
	return nil, 0, nil
}

// mock publisher
type mockPub struct {
	last event.Event
}

func (m *mockPub) Publish(ctx context.Context, e event.Event) error { m.last = e; return nil }
func (m *mockPub) Close() error                                     { return nil }

func TestFriendship_SendAcceptRejectCancelRemove_IsFriend(t *testing.T) {
	ctx := context.Background()
	repo := newMockFriendRepo()
	pub := &mockPub{}
	svc := service2.NewFriendshipService(repo, pub)

	// send request
	f, err := svc.SendRequest(ctx, 1, 2)
	if err != nil {
		t.Fatalf("SendRequest failed: %v", err)
	}
	if f.RequesterID != 1 || f.AddresseeID != 2 || f.Status != friendship.StatusPending {
		t.Fatalf("unexpected friendship created: %+v", f)
	}
	if pub.last.Name != event.FriendRequestReceived {
		t.Fatalf("expected FriendRequestReceived published")
	}

	// accept
	acc, err := svc.Accept(ctx, 2, f.ID)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	if acc.Status != friendship.StatusAccepted {
		t.Fatalf("expected accepted, got %s", acc.Status)
	}
	if pub.last.Name != event.FriendRequestAccepted {
		t.Fatalf("expected FriendRequestAccepted published")
	}

	// is friend
	isf, err := svc.IsFriend(ctx, 1, 2)
	if err != nil || !isf {
		t.Fatalf("IsFriend expected true")
	}

	// remove
	if err := svc.Remove(ctx, 1, f.ID); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// now not friend
	isf2, _ := svc.IsFriend(ctx, 1, 2)
	if isf2 {
		t.Fatalf("IsFriend expected false after remove")
	}

	// send another and cancel
	f2, _ := svc.SendRequest(ctx, 1, 3)
	if err := svc.Cancel(ctx, 1, f2.ID); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	// reject by addressee
	f3, _ := svc.SendRequest(ctx, 4, 5)
	_, err = svc.Reject(ctx, 5, f3.ID)
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	// cannot friend yourself
	_, err = svc.SendRequest(ctx, 6, 6)
	if !errors.Is(err, errs.ErrCannotFriendYourself) {
		t.Fatalf("expected ErrCannotFriendYourself, got %v", err)
	}
}
