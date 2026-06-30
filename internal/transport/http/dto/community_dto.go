package dto

import "time"

// ── request DTOs ─────────────────────────────────────────────────────────────

// CreateCommunityRequest is the body for POST /api/v1/communities.
type CreateCommunityRequest struct {
	Type        string `json:"type"`
	Visibility  string `json:"visibility"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

// UpdateCommunityRequest is the body for PATCH /api/v1/communities/{id}.
type UpdateCommunityRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AvatarKey   string `json:"avatar_key"`
	BannerKey   string `json:"banner_key"`
}

// ChangeCommunityTypeRequest is the body for PATCH /api/v1/communities/{id}/type.
type ChangeCommunityTypeRequest struct {
	Type string `json:"type"`
}

// SetMemberRoleRequest is the body for POST /api/v1/communities/{id}/members/{userID}/role.
type SetMemberRoleRequest struct {
	Role string `json:"role"`
}

// ── response DTOs ─────────────────────────────────────────────────────────────

// CommunityResponse is the public representation of a community.
type CommunityResponse struct {
	ID               int64     `json:"id"`
	OwnerID          int64     `json:"owner_id"`
	Type             string    `json:"type"`
	Visibility       string    `json:"visibility"`
	Name             string    `json:"name"`
	Slug             string    `json:"slug"`
	Description      string    `json:"description"`
	AvatarURL        string    `json:"avatar_url,omitempty"`
	BannerURL        string    `json:"banner_url,omitempty"`
	MembersCount     int64     `json:"members_count"`
	PostsCount       int64     `json:"posts_count"`
	SubscribersCount int64     `json:"subscribers_count"`
	VideosCount      int64     `json:"videos_count"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CommunityMemberResponse is a single community membership record.
type CommunityMemberResponse struct {
	CommunityID int64     `json:"community_id"`
	UserID      int64     `json:"user_id"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

// CommunityListResponse wraps a slice of communities.
type CommunityListResponse struct {
	Communities []*CommunityResponse `json:"communities"`
}

// CommunityMemberListResponse wraps a slice of members.
type CommunityMemberListResponse struct {
	Members []*CommunityMemberResponse `json:"members"`
}
