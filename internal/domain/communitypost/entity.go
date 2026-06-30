package communitypost

import "time"

// MediaItem mirrors userpost.MediaItem but is duplicated here intentionally —
// domain packages must not import each other (see AGENTS.md §1).
type MediaItem struct {
	Type       string  `json:"type"` // image | video | audio
	StorageKey string  `json:"storage_key"`
	Width      int     `json:"width,omitempty"`
	Height     int     `json:"height,omitempty"`
	DurationS  float64 `json:"duration_s,omitempty"`
}

// Post is a post published from a community's identity by an admin/owner.
type Post struct {
	ID           int64
	CommunityID  int64
	AuthorUserID int64 // the admin/owner who actually clicked "publish"
	Text         string
	Media        []MediaItem

	// VideoID links this post to a published video. Set by the Stage 4
	// bridge in VideoService.Publish when "video_feed" is one of the
	// publish targets — see AGENTS.md "Communities Feature Guidance".
	VideoID *int64

	Pinned bool

	LikesCount    int64
	CommentsCount int64

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time // soft-delete
}
