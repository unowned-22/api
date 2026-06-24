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
	// Device metadata
	DeviceName *string
	DeviceOS   *string
	DeviceType *string

	// Geolocation
	Latitude     *float64
	Longitude    *float64
	LocationName *string

	// EXIF raw JSON
	ExifData []byte

	// Counters & viewer-specific
	LikesCount    int
	CommentsCount int
	IsLiked       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
