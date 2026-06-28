package dto

import "encoding/json"

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type UserResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FullName  string `json:"full_name"`
	Username  string `json:"username"`
	Phone     string `json:"phone"`
	AvatarURL string `json:"avatar_url"`
	CoverURL  string `json:"cover_url"`
	CreatedAt string `json:"created_at"`
}

type PresignedUploadResponse struct {
	UploadURL string `json:"upload_url"`
	Key       string `json:"key"`
	ExpiresIn int    `json:"expires_in"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type SessionResponse struct {
	ID             int64  `json:"id"`
	UserID         int64  `json:"user_id"`
	DeviceName     string `json:"device_name"`
	Browser        string `json:"browser"`
	OS             string `json:"os"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
	LastActivityAt string `json:"last_activity_at"`
	ExpiresAt      string `json:"expires_at"`
}

type SessionListResponse struct {
	Sessions []SessionResponse `json:"sessions"`
}

type UserSettingsResponse struct {
	UserID            int64           `json:"user_id"`
	StorageQuotaBytes int64           `json:"storage_quota_bytes"`
	StorageUsedBytes  int64           `json:"storage_used_bytes"`
	BucketName        string          `json:"bucket_name"`
	Theme             json.RawMessage `json:"theme"`
	UpdatedAt         string          `json:"updated_at"`
}

type StoryResponse struct {
	ID             int64             `json:"id"`
	UserID         int64             `json:"user_id"`
	AuthorName     string            `json:"author_name"`
	AuthorUsername string            `json:"author_username"`
	AuthorAvatar   string            `json:"author_avatar"`
	Visibility     string            `json:"visibility"`
	Duration       int               `json:"duration"`
	HiddenFrom     []int64           `json:"hidden_from"`
	Slides         []json.RawMessage `json:"slides"`
	CreatedAt      string            `json:"created_at"`
	ExpiresAt      string            `json:"expires_at"`
}

type StoryMediaResponse struct {
	URL       string `json:"url"`
	Key       string `json:"key"`
	MediaType string `json:"media_type"`
}

type LinkZone struct {
	URL          string  `json:"url"`
	DisplayStyle string  `json:"display_style"` // "pill" | "card"
	Title        string  `json:"title,omitempty"`
	X            float64 `json:"x"`      // centre-x, % of canvas width
	Y            float64 `json:"y"`      // centre-y, % of canvas height
	Width        float64 `json:"width"`  // element width, % of canvas width
	Height       float64 `json:"height"` // estimated height, % of canvas height
	Rotation     float64 `json:"rotation"`
}

type FeedSlideResponse struct {
	ID          string     `json:"id"`
	RenderedURL string     `json:"rendered_url,omitempty"`
	Seen        bool       `json:"seen"`
	LinkZones   []LinkZone `json:"link_zones,omitempty"`
}

type CoverUploadResponse struct {
	OriginalURL string `json:"cover_url"`
	MobileURL   string `json:"cover_mobile_url"`
	DesktopURL  string `json:"cover_desktop_url"`
}
