package dto

import "time"

// ── requests ─────────────────────────────────────────────────────────────────

// MediaItemRequest is a single attached media object in a post request.
type MediaItemRequest struct {
	Type       string  `json:"type"`
	StorageKey string  `json:"storage_key"`
	Width      int     `json:"width,omitempty"`
	Height     int     `json:"height,omitempty"`
	DurationS  float64 `json:"duration_s,omitempty"`
}

// CreatePostRequest is the body for POST /api/v1/posts.
type CreatePostRequest struct {
	AuthorType  string             `json:"author_type"` // "user" | "community"
	CommunityID *int64             `json:"community_id,omitempty"`
	Text        string             `json:"text"`
	Media       []MediaItemRequest `json:"media"`
	Visibility  string             `json:"visibility,omitempty"` // user posts only
}

// ── responses ─────────────────────────────────────────────────────────────────

// MediaItemResponse mirrors MediaItemRequest for output.
type MediaItemResponse struct {
	Type       string  `json:"type"`
	StorageKey string  `json:"storage_key"`
	Width      int     `json:"width,omitempty"`
	Height     int     `json:"height,omitempty"`
	DurationS  float64 `json:"duration_s,omitempty"`
}

// UserPostResponse is the public representation of a personal post.
type UserPostResponse struct {
	ID            int64               `json:"id"`
	UserID        int64               `json:"user_id"`
	Text          string              `json:"text"`
	Media         []MediaItemResponse `json:"media"`
	Visibility    string              `json:"visibility"`
	LikesCount    int64               `json:"likes_count"`
	CommentsCount int64               `json:"comments_count"`
	CreatedAt     time.Time           `json:"created_at"`
}

// CommunityPostResponse is the public representation of a community post.
type CommunityPostResponse struct {
	ID            int64               `json:"id"`
	CommunityID   int64               `json:"community_id"`
	AuthorUserID  int64               `json:"author_user_id"`
	Text          string              `json:"text"`
	Media         []MediaItemResponse `json:"media"`
	VideoID       *int64              `json:"video_id,omitempty"`
	Pinned        bool                `json:"pinned"`
	LikesCount    int64               `json:"likes_count"`
	CommentsCount int64               `json:"comments_count"`
	CreatedAt     time.Time           `json:"created_at"`
}

// CreatePostResponse wraps the result of POST /api/v1/posts — exactly one
// of user_post / community_post is set, discriminated by source_type.
type CreatePostResponse struct {
	SourceType    string                 `json:"source_type"`
	UserPost      *UserPostResponse      `json:"user_post,omitempty"`
	CommunityPost *CommunityPostResponse `json:"community_post,omitempty"`
}

// FeedItemResponse is a single row in the home feed response.
type FeedItemResponse struct {
	SourceType    string              `json:"source_type"`
	ID            int64               `json:"id"`
	OwnerID       int64               `json:"owner_id"`
	CommunityID   *int64              `json:"community_id,omitempty"`
	Text          string              `json:"text"`
	Media         []MediaItemResponse `json:"media"`
	LikesCount    int64               `json:"likes_count"`
	CommentsCount int64               `json:"comments_count"`
	CreatedAt     time.Time           `json:"created_at"`
}

// FeedResponse wraps the home feed listing.
type FeedResponse struct {
	Items []*FeedItemResponse `json:"items"`
}
