package userpost

import "time"

// Visibility controls who can see a personal post.
type Visibility string

const (
	VisibilityEveryone Visibility = "everyone"
	VisibilityFriends  Visibility = "friends"
	VisibilityPrivate  Visibility = "private"
)

// MediaItem is a single attached media object (image/video/audio).
// Stored as JSONB in user_posts.media — opaque beyond this shape,
// following the same pattern as story slides (see AGENTS.md "Stories").
type MediaItem struct {
	Type       string  `json:"type"` // image | video | audio
	StorageKey string  `json:"storage_key"`
	Width      int     `json:"width,omitempty"`
	Height     int     `json:"height,omitempty"`
	DurationS  float64 `json:"duration_s,omitempty"`
}

// Post is a personal (user-authored) post.
type Post struct {
	ID            int64
	UserID        int64
	Text          string
	Media         []MediaItem
	Visibility    Visibility
	LikesCount    int64
	CommentsCount int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time // soft-delete
}
