package video

import "time"

type Visibility string
type Status string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityUnlisted Visibility = "unlisted"
	VisibilityPrivate  Visibility = "private"

	// Pipeline statuses (set by the async worker)
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusReady      Status = "ready"
	StatusFailed     Status = "failed"

	// StatusArchived is set when the parent community is soft-deleted
	// with type=video. The video files are NOT removed from object storage.
	StatusArchived Status = "archived"
)

// PublishTarget enumerates where a published video appears.
type PublishTarget = string

const (
	PublishTargetVideoFeed     PublishTarget = "video_feed"     // appears in subscriber feed
	PublishTargetCommunityPage PublishTarget = "community_page" // visible on community page only
)

type Video struct {
	ID            int64
	CommunityID   int64 // was ChannelID — renamed to match communities.id FK
	UserID        int64
	Title         string
	Description   string
	Category      string
	Tags          []string
	Visibility    Visibility
	Status        Status
	RawKey        string
	HLSKey        string
	MP4360Key     string
	MP4720Key     string
	ThumbnailKey  string
	CoverKey      string
	DurationSec   float64
	Width         int
	Height        int
	SizeBytes     int64
	VideoCodec    string
	AudioCodec    string
	ViewsCount    int64
	LikesCount    int64
	CommentsCount int64
	IsLiked       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Processing progress fields — only meaningful while Status == "processing".
	ProcessingStage     string
	ProcessingProgress  int
	ProcessingStartedAt *time.Time

	// Publish lifecycle — nil means the video is still a draft (only visible
	// to the owner/admins of the community via the drafts endpoint).
	PublishedAt    *time.Time
	PublishTargets []string // subset of PublishTarget constants

	// BoostedUntil is a no-op stub for the future promotion subsystem.
	// TODO: business logic for boosting/ranking is out of scope for this task
	//       and will be implemented in a separate story (see TASK-communities §4, §10).
	BoostedUntil *time.Time
}
