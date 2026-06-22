package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/unowned-22/api/internal/domain/usersettings"
)

type userSettingsService struct {
	repo usersettings.Repository
}

func NewUserSettingsService(repo usersettings.Repository) usersettings.Service {
	return &userSettingsService{repo: repo}
}

func (s *userSettingsService) GetMySettings(ctx context.Context, userID int64) (*usersettings.UserSettings, error) {
	st, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if st == nil {
		// create defaults
		def := &usersettings.UserSettings{
			UserID:            userID,
			StorageQuotaBytes: 1073741824,
			StorageUsedBytes:  0,
			BucketName:        "",
			Theme:             json.RawMessage([]byte(`{}`)),
		}
		if err := s.repo.Create(ctx, def); err != nil {
			return nil, err
		}
		return def, nil
	}
	return st, nil
}

func (s *userSettingsService) UpdateMyTheme(ctx context.Context, userID int64, theme json.RawMessage) error {
	// validate JSON
	var tmp interface{}
	if err := json.Unmarshal(theme, &tmp); err != nil {
		return fmt.Errorf("invalid theme JSON: %w", err)
	}
	return s.repo.UpdateTheme(ctx, userID, theme)
}

func (s *userSettingsService) GetNotificationPreferences(ctx context.Context, userID int64) (json.RawMessage, error) {
	prefs, err := s.repo.GetNotificationPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}
	if prefs == nil || len(prefs) == 0 {
		// default preferences: fail-open (true)
		def := json.RawMessage([]byte(`{"story_published": true, "friend_request_received": true, "friend_request_accepted": true}`))
		return def, nil
	}
	return prefs, nil
}

func (s *userSettingsService) UpdateMyNotificationPreferences(ctx context.Context, userID int64, prefs json.RawMessage) error {
	// validate JSON
	var tmp interface{}
	if err := json.Unmarshal(prefs, &tmp); err != nil {
		return fmt.Errorf("invalid notification preferences JSON: %w", err)
	}
	return s.repo.UpdateNotificationPreferences(ctx, userID, prefs)
}

func (s *userSettingsService) GetUserSettings(ctx context.Context, userID int64) (*usersettings.UserSettings, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *userSettingsService) UpdateUserQuota(ctx context.Context, userID int64, quotaBytes int64) error {
	return s.repo.UpdateQuota(ctx, userID, quotaBytes)
}

func (s *userSettingsService) UpdateBucketName(ctx context.Context, userID int64, bucketName string) error {
	return s.repo.UpdateBucketName(ctx, userID, bucketName)
}
