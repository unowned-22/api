package dto

// PromoteToCommunityRequest is the body for
// POST /api/v1/conversations/{id}/promote-to-community.
type PromoteToCommunityRequest struct {
	// Type is the resulting community.Type (e.g. "general", "blog", "news").
	Type string `json:"type"`
	// Visibility is "public" or "private". Determines whether the
	// auto-linked conversation keeps acting as a channel (public) or
	// stays a private group.
	Visibility string `json:"visibility"`
}

// PromoteToCommunityResponse wraps the resulting community ID.
type PromoteToCommunityResponse struct {
	CommunityID int64 `json:"community_id"`
}
