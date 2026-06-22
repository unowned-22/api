package service

import (
	"context"

	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/pagination"
)

type notificationService struct {
	repo notification.Repository
}

func NewNotificationService(repo notification.Repository) notification.Service {
	return &notificationService{repo: repo}
}

func (s *notificationService) ListMy(ctx context.Context, userID int64, page pagination.Query) ([]*notification.Notification, int64, error) {
	return s.repo.ListByUser(ctx, userID, page)
}

func (s *notificationService) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	return s.repo.CountUnread(ctx, userID)
}

func (s *notificationService) MarkRead(ctx context.Context, userID int64, id int64) error {
	return s.repo.MarkRead(ctx, userID, id)
}

func (s *notificationService) MarkAllRead(ctx context.Context, userID int64) error {
	return s.repo.MarkAllRead(ctx, userID)
}
