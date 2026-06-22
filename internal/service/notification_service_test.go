package service

import (
	"context"
	"testing"
	"time"

	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/pagination"
)

type mockNotifRepo struct {
	items []*notification.Notification
}

func (m *mockNotifRepo) Create(ctx context.Context, n *notification.Notification) error {
	n.ID = int64(len(m.items) + 1)
	m.items = append(m.items, n)
	return nil
}

func (m *mockNotifRepo) CreateBatch(ctx context.Context, ns []*notification.Notification) error {
	for _, n := range ns {
		n.ID = int64(len(m.items) + 1)
		m.items = append(m.items, n)
	}
	return nil
}

func (m *mockNotifRepo) ListByUser(ctx context.Context, userID int64, page pagination.Query) ([]*notification.Notification, int64, error) {
	var out []*notification.Notification
	for _, it := range m.items {
		if it.UserID == userID {
			out = append(out, it)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockNotifRepo) MarkRead(ctx context.Context, userID int64, notificationID int64) error {
	for _, it := range m.items {
		if it.ID == notificationID && it.UserID == userID {
			it.IsRead = true
			return nil
		}
	}
	return nil
}

func (m *mockNotifRepo) MarkAllRead(ctx context.Context, userID int64) error {
	for _, it := range m.items {
		if it.UserID == userID {
			it.IsRead = true
		}
	}
	return nil
}

func (m *mockNotifRepo) CountUnread(ctx context.Context, userID int64) (int64, error) {
	var c int64
	for _, it := range m.items {
		if it.UserID == userID && !it.IsRead {
			c++
		}
	}
	return c, nil
}

func TestNotificationService_Basic(t *testing.T) {
	repo := &mockNotifRepo{}
	svc := NewNotificationService(repo)

	ctx := context.Background()

	n := &notification.Notification{UserID: 1, ActorID: 2, Type: notification.TypeFriendRequestReceived, EntityType: "friend_request", EntityID: 10, Payload: nil, IsRead: false, CreatedAt: time.Now()}
	if err := svc.MarkAllRead(ctx, 1); err != nil {
		t.Fatalf("MarkAllRead failed: %v", err)
	}
	if err := repo.Create(ctx, n); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	list, total, err := svc.ListMy(ctx, 1, pagination.Query{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("ListMy failed: %v", err)
	}
	if total != int64(len(list)) {
		t.Fatalf("unexpected total: %d vs %d", total, len(list))
	}

	c, err := svc.UnreadCount(ctx, 1)
	if err != nil {
		t.Fatalf("UnreadCount failed: %v", err)
	}
	if c != 1 {
		t.Fatalf("expected unread 1, got %d", c)
	}

	if err := svc.MarkRead(ctx, 1, n.ID); err != nil {
		t.Fatalf("MarkRead failed: %v", err)
	}
	c2, _ := svc.UnreadCount(ctx, 1)
	if c2 != 0 {
		t.Fatalf("expected unread 0 after mark, got %d", c2)
	}
}
