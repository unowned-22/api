package album

import "time"

type Visibility string

const (
	VisibilityEveryone Visibility = "everyone"
	VisibilityFriends  Visibility = "friends"
	VisibilityNobody   Visibility = "nobody"
)

type Album struct {
	ID           int64
	UserID       int64
	Title        string
	Description  string
	Visibility   Visibility
	HiddenFrom   []int64
	CoverPhotoID *int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
