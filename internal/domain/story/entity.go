package story

import "time"

type Visibility string

const (
	VisibilityEveryone Visibility = "everyone"
	VisibilityFriends  Visibility = "friends"
	VisibilityClose    Visibility = "close"
)

const (
	AuthorTypeUser      = "user"
	AuthorTypeCommunity = "community"
)

type Story struct {
	ID                int64
	UserID            int64
	Visibility        Visibility
	AuthorType        string
	CommunityID       *int64
	DurationHours     int
	HiddenFromUserIDs []int64
	Slides            []byte // raw JSON array
	CreatedAt         time.Time
	ExpiresAt         time.Time
}
