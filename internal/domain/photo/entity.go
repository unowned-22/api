package photo

import "time"

type Visibility string

const (
	VisibilityEveryone Visibility = "everyone"
	VisibilityFriends  Visibility = "friends"
	VisibilityNobody   Visibility = "nobody"
)

type Photo struct {
	ID          int64
	UserID      int64
	AlbumID     *int64
	DisplayName string
	StorageKey  string
	URL         string
	SizeBytes   int64
	Width       *int
	Height      *int
	MimeType    string
	Visibility  Visibility
	HiddenFrom  []int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
