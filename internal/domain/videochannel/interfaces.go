package videochannel

import "context"

type Repository interface {
	Create(ctx context.Context, c *Channel) error
	GetByID(ctx context.Context, id int64) (*Channel, error)
	GetByUserID(ctx context.Context, userID int64) (*Channel, error)
	Update(ctx context.Context, c *Channel) error
	IncrVideosCount(ctx context.Context, id int64) error
	DecrVideosCount(ctx context.Context, id int64) error
	IncrSubscribers(ctx context.Context, id int64) error
	DecrSubscribers(ctx context.Context, id int64) error
}

type Service interface {
	GetOrCreate(ctx context.Context, userID int64, fullName string) (*Channel, error)
	GetChannel(ctx context.Context, id int64) (*Channel, error)
	GetChannelByUser(ctx context.Context, userID int64) (*Channel, error)
	UpdateChannel(ctx context.Context, channelID int64, requesterID int64, req UpdateRequest) (*Channel, error)
}

type UpdateRequest struct {
	Name        string
	Description string
	AvatarKey   string
	BannerKey   string
}
