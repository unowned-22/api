package dto

import "time"

type ChannelResponse struct {
	ID               int64     `json:"id"`
	UserID           int64     `json:"user_id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	AvatarURL        string    `json:"avatar_url"`
	BannerURL        string    `json:"banner_url"`
	SubscribersCount int64     `json:"subscribers_count"`
	VideosCount      int64     `json:"videos_count"`
	IsSubscribed     bool      `json:"is_subscribed"`
	CreatedAt        time.Time `json:"created_at"`
}

type UpdateChannelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AvatarKey   string `json:"avatar_key"`
	BannerKey   string `json:"banner_key"`
}
type VideoResponse struct {
	ID            int64     `json:"id"`
	ChannelID     int64     `json:"channel_id"`
	UserID        int64     `json:"user_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Category      string    `json:"category"`
	Tags          []string  `json:"tags"`
	Visibility    string    `json:"visibility"`
	Status        string    `json:"status"`
	CoverURL      string    `json:"cover_url"`
	ThumbnailURL  string    `json:"thumbnail_url,omitempty"`
	MP4360URL     string    `json:"mp4_360_url"`
	MP4720URL     string    `json:"mp4_720_url,omitempty"`
	DurationSec   float64   `json:"duration_sec"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	ViewsCount    int64     `json:"views_count"`
	LikesCount    int64     `json:"likes_count"`
	CommentsCount int64     `json:"comments_count"`
	IsLiked       bool      `json:"is_liked"`
	CreatedAt     time.Time `json:"created_at"`
}
type VideoListResponse struct {
	Videos []*VideoResponse `json:"videos"`
	Total  int              `json:"total"`
}
type UpdateVideoRequest struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	Tags         []string `json:"tags"`
	Visibility   string   `json:"visibility"`
	ThumbnailKey string   `json:"thumbnail_key,omitempty"`
}
type VideoCommentResponse struct {
	ID         int64     `json:"id"`
	VideoID    int64     `json:"video_id"`
	UserID     int64     `json:"user_id"`
	ParentID   *int64    `json:"parent_id,omitempty"`
	Body       string    `json:"body"`
	LikesCount int64     `json:"likes_count"`
	IsLiked    bool      `json:"is_liked"`
	CreatedAt  time.Time `json:"created_at"`
}
type VideoCommentListResponse struct {
	Comments []*VideoCommentResponse `json:"comments"`
	Total    int                     `json:"total"`
}
type CreateVideoCommentRequest struct {
	ParentID *int64 `json:"parent_id,omitempty"`
	Body     string `json:"body"`
}
type PlaylistResponse struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Visibility  string    `json:"visibility"`
	ItemsCount  int64     `json:"items_count"`
	CreatedAt   time.Time `json:"created_at"`
}
type CreatePlaylistRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
}
type UpdatePlaylistRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
}
type AddPlaylistItemRequest struct {
	VideoID int64 `json:"video_id"`
}

type PlaylistItemResponse struct {
	ID         int64     `json:"id"`
	PlaylistID int64     `json:"playlist_id"`
	VideoID    int64     `json:"video_id"`
	Position   int       `json:"position"`
	AddedAt    time.Time `json:"added_at"`
}

type PlaylistItemListResponse struct {
	Items  []PlaylistItemResponse `json:"items"`
	Total  int                    `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
}
