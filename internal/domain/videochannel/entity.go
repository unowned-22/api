package videochannel

import "time"

type Channel struct {
	ID               int64
	UserID           int64
	Name             string
	Description      string
	AvatarKey        string
	BannerKey        string
	SubscribersCount int64
	VideosCount      int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
