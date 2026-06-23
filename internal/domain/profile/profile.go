package profile

import (
	"context"
	"time"
)

// Relation describes viewer -> target relation
type Relation string

const (
	RelationSelf            Relation = "self"
	RelationFriends         Relation = "friends"
	RelationOutgoingRequest Relation = "outgoing_request"
	RelationIncomingRequest Relation = "incoming_request"
	RelationNone            Relation = "none"
)

// PublicProfile is returned after applying privacy rules
type PublicProfile struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	FullName     string    `json:"full_name"`
	AvatarURL    string    `json:"avatar_url"`
	CoverURL     string    `json:"cover_url"`
	Email        *string   `json:"email,omitempty"`
	Phone        *string   `json:"phone,omitempty"`
	FriendsCount *int64    `json:"friends_count,omitempty"`
	Relation     Relation  `json:"relation"`
	CreatedAt    time.Time `json:"created_at"`
}

type Service interface {
	GetPublicProfile(ctx context.Context, viewerID int64, username string) (*PublicProfile, error)
}
