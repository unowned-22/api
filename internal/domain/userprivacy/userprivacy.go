package userprivacy

import (
	"context"
	"time"
)

type VisibilityLevel string

const (
	VisibilityEveryone VisibilityLevel = "everyone"
	VisibilityFriends  VisibilityLevel = "friends"
	VisibilityNobody   VisibilityLevel = "nobody"
)

type UserPrivacySettings struct {
	UserID      int64           `json:"user_id"`
	ShowEmail   VisibilityLevel `json:"show_email"`
	ShowPhone   VisibilityLevel `json:"show_phone"`
	ShowFriends VisibilityLevel `json:"show_friends"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type Repository interface {
	// GetByUserID returns privacy settings; if not found — returns default settings (not an error)
	GetByUserID(ctx context.Context, userID int64) (*UserPrivacySettings, error)
	// Upsert creates or updates settings
	Upsert(ctx context.Context, settings *UserPrivacySettings) error
}

func Default(userID int64) *UserPrivacySettings {
	return &UserPrivacySettings{
		UserID:      userID,
		ShowEmail:   VisibilityNobody,
		ShowPhone:   VisibilityNobody,
		ShowFriends: VisibilityEveryone,
	}
}
