package service

import (
	"context"

	"github.com/unowned-22/api/internal/domain/videochannel"
	"github.com/unowned-22/api/internal/domain/videosubscription"
	"github.com/unowned-22/api/internal/errs"
)

type VideoSubscriptionService struct {
	subRepo     videosubscription.Repository
	channelRepo videochannel.Repository
}

func NewVideoSubscriptionService(sub videosubscription.Repository, ch videochannel.Repository) *VideoSubscriptionService {
	return &VideoSubscriptionService{subRepo: sub, channelRepo: ch}
}

func (s *VideoSubscriptionService) Subscribe(ctx context.Context, subscriberID, channelID int64) error {
	ch, err := s.channelRepo.GetByID(ctx, channelID)
	if err != nil {
		return err
	}
	if ch.UserID == subscriberID {
		return errs.ErrCannotSubscribeSelf
	}
	return s.subRepo.Subscribe(ctx, subscriberID, channelID)
}
func (s *VideoSubscriptionService) Unsubscribe(ctx context.Context, subscriberID, channelID int64) error {
	return s.subRepo.Unsubscribe(ctx, subscriberID, channelID)
}
func (s *VideoSubscriptionService) IsSubscribed(ctx context.Context, subscriberID, channelID int64) (bool, error) {
	return s.subRepo.IsSubscribed(ctx, subscriberID, channelID)
}
func (s *VideoSubscriptionService) ListSubscribedChannels(ctx context.Context, subscriberID int64) ([]int64, error) {
	return s.subRepo.ListSubscribedChannelIDs(ctx, subscriberID)
}

var _ videosubscription.Service = (*VideoSubscriptionService)(nil)
