package videosubscription

import (
	"context"
	"time"
)

type Subscription struct {
	SubscriberID int64
	ChannelID    int64
	CreatedAt    time.Time
}

type Repository interface {
	Subscribe(ctx context.Context, subscriberID, channelID int64) error
	Unsubscribe(ctx context.Context, subscriberID, channelID int64) error
	IsSubscribed(ctx context.Context, subscriberID, channelID int64) (bool, error)
	ListSubscriberIDs(ctx context.Context, channelID int64) ([]int64, error)
	ListSubscribedChannelIDs(ctx context.Context, subscriberID int64) ([]int64, error)
}

type Service interface {
	Subscribe(ctx context.Context, subscriberID, channelID int64) error
	Unsubscribe(ctx context.Context, subscriberID, channelID int64) error
	IsSubscribed(ctx context.Context, subscriberID, channelID int64) (bool, error)
	ListSubscribedChannels(ctx context.Context, subscriberID int64) ([]int64, error)
}
