package videoplaylist

import "time"

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

type Playlist struct {
	ID          int64
	UserID      int64
	Title       string
	Description string
	Visibility  Visibility
	ItemsCount  int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PlaylistItem struct {
	ID         int64
	PlaylistID int64
	VideoID    int64
	Position   int
	AddedAt    time.Time
}
