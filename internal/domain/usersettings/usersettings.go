package usersettings

import (
	"context"
	"encoding/json"
	"time"
)

// UserSettings represents per-user configurable settings and usage.
type UserSettings struct {
	UserID                  int64           `json:"user_id"`
	StorageQuotaBytes       int64           `json:"storage_quota_bytes"`
	StorageUsedBytes        int64           `json:"storage_used_bytes"`
	BucketName              string          `json:"bucket_name"`
	Theme                   json.RawMessage `json:"theme"`
	NotificationPreferences json.RawMessage `json:"notification_preferences"`
	UpdatedAt               time.Time       `json:"updated_at"`
}

type Repository interface {
	Create(ctx context.Context, settings *UserSettings) error
	GetByUserID(ctx context.Context, userID int64) (*UserSettings, error)
	UpdateTheme(ctx context.Context, userID int64, theme json.RawMessage) error
	UpdateQuota(ctx context.Context, userID int64, quotaBytes int64) error
	UpdateBucketName(ctx context.Context, userID int64, bucketName string) error
	IncrementUsedBytes(ctx context.Context, userID int64, delta int64) error
	UpdateNotificationPreferences(ctx context.Context, userID int64, prefs json.RawMessage) error
	GetNotificationPreferences(ctx context.Context, userID int64) (json.RawMessage, error)
}

type Service interface {
	GetMySettings(ctx context.Context, userID int64) (*UserSettings, error)
	UpdateMyTheme(ctx context.Context, userID int64, theme json.RawMessage) error
	GetUserSettings(ctx context.Context, userID int64) (*UserSettings, error)
	UpdateUserQuota(ctx context.Context, userID int64, quotaBytes int64) error
	UpdateBucketName(ctx context.Context, userID int64, bucketName string) error
	GetNotificationPreferences(ctx context.Context, userID int64) (json.RawMessage, error)
	UpdateMyNotificationPreferences(ctx context.Context, userID int64, prefs json.RawMessage) error
}
