package service

import (
	"context"
	"strings"

	"github.com/unowned-22/api/internal/domain/videochannel"
	"github.com/unowned-22/api/internal/errs"
)

type VideoChannelService struct{ channelRepo videochannel.Repository }

func NewVideoChannelService(repo videochannel.Repository) *VideoChannelService {
	return &VideoChannelService{channelRepo: repo}
}

func (s *VideoChannelService) GetOrCreate(ctx context.Context, userID int64, fullName string) (*videochannel.Channel, error) {
	if c, err := s.channelRepo.GetByUserID(ctx, userID); err == nil {
		return c, nil
	}
	c := &videochannel.Channel{UserID: userID, Name: strings.TrimSpace(fullName)}
	if c.Name == "" {
		c.Name = "Channel"
	}
	if err := s.channelRepo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}
func (s *VideoChannelService) GetChannel(ctx context.Context, id int64) (*videochannel.Channel, error) {
	return s.channelRepo.GetByID(ctx, id)
}
func (s *VideoChannelService) GetChannelByUser(ctx context.Context, userID int64) (*videochannel.Channel, error) {
	return s.channelRepo.GetByUserID(ctx, userID)
}
func (s *VideoChannelService) UpdateChannel(ctx context.Context, channelID int64, requesterID int64, req videochannel.UpdateRequest) (*videochannel.Channel, error) {
	c, err := s.channelRepo.GetByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if c.UserID != requesterID {
		return nil, errs.ErrForbidden
	}
	c.Name, c.Description, c.AvatarKey, c.BannerKey = req.Name, req.Description, req.AvatarKey, req.BannerKey
	return c, s.channelRepo.Update(ctx, c)
}

var _ videochannel.Service = (*VideoChannelService)(nil)
