package video

import "time"

type Visibility string
type Status string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityUnlisted Visibility = "unlisted"
	VisibilityPrivate  Visibility = "private"

	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusReady      Status = "ready"
	StatusFailed     Status = "failed"
)

type Video struct {
	ID            int64
	ChannelID     int64
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

	ProcessingStage     string
	ProcessingProgress  int
	ProcessingStartedAt *time.Time
}
