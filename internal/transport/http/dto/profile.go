package dto

type PublicProfileResponse struct {
	ID           int64   `json:"id"`
	Username     string  `json:"username"`
	FullName     string  `json:"full_name"`
	AvatarURL    string  `json:"avatar_url"`
	CoverURL     string  `json:"cover_url"`
	Email        *string `json:"email,omitempty"`
	Phone        *string `json:"phone,omitempty"`
	FriendsCount *int64  `json:"friends_count,omitempty"`
	Relation     string  `json:"relation"`
	CreatedAt    string  `json:"created_at"`
}
