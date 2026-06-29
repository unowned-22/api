package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/unowned-22/api/internal/domain/media"
	domainstorage "github.com/unowned-22/api/internal/domain/storage"
	"github.com/unowned-22/api/internal/domain/videochannel"
	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/validator"
)

type VideoChannelService struct {
	channelRepo  videochannel.Repository
	storage      domainstorage.Storage
	publicBucket string
}

func NewVideoChannelService(repo videochannel.Repository, storage domainstorage.Storage, publicBucket string) *VideoChannelService {
	return &VideoChannelService{channelRepo: repo, storage: storage, publicBucket: publicBucket}
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

func (s *VideoChannelService) CreateChannel(ctx context.Context, userID int64, req videochannel.CreateRequest) (*videochannel.Channel, error) {
	if _, err := s.channelRepo.GetByUserID(ctx, userID); err == nil {
		return nil, errs.ErrChannelAlreadyExists
	} else if err != nil && !errors.Is(err, errs.ErrChannelNotFound) {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	description := strings.TrimSpace(req.Description)
	if err := validator.Validate(struct {
		Name        string `validate:"required,min=3,max=100"`
		Description string `validate:"omitempty,max=500"`
	}{Name: name, Description: description}); err != nil {
		return nil, err
	}

	c := &videochannel.Channel{UserID: userID, Name: name, Description: description}
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

func (s *VideoChannelService) UploadAvatar(ctx context.Context, userID int64, data []byte, size int64, contentType string) (*videochannel.Channel, error) {
	return s.uploadAsset(ctx, userID, data, size, contentType, "avatar", 5*1024*1024)
}

func (s *VideoChannelService) UploadBanner(ctx context.Context, userID int64, data []byte, size int64, contentType string) (*videochannel.Channel, error) {
	return s.uploadAsset(ctx, userID, data, size, contentType, "banner", 10*1024*1024)
}

func (s *VideoChannelService) uploadAsset(ctx context.Context, userID int64, data []byte, size int64, contentType, assetType string, maxSize int64) (*videochannel.Channel, error) {
	channel, err := s.channelRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if channel.UserID != userID {
		return nil, errs.ErrForbidden
	}
	if size > maxSize {
		return nil, fmt.Errorf("%w: %s exceeds maximum allowed size", errs.ErrStorageQuotaExceeded, assetType)
	}
	f, err := media.DetectFormat(data)
	if err != nil {
		return nil, errs.ErrUnsupportedVideoType
	}
	allowedFormats := map[media.Format]bool{media.FormatJPEG: true, media.FormatPNG: true, media.FormatWebP: true}
	if !allowedFormats[f] {
		return nil, errs.ErrUnsupportedVideoType
	}
	ext := media.FormatExtension(f)
	key := fmt.Sprintf("channels/%d/%s/%s.%s", channel.ID, assetType, assetType, ext)
	urlPath, err := s.storage.PutObject(ctx, s.publicBucket, key, bytes.NewReader(data), size, contentType)
	if err != nil {
		return nil, err
	}
	if assetType == "avatar" {
		channel.AvatarKey = urlPath
	} else {
		channel.BannerKey = urlPath
	}
	if err := s.channelRepo.Update(ctx, channel); err != nil {
		return nil, err
	}
	return channel, nil
}

var _ videochannel.Service = (*VideoChannelService)(nil)
