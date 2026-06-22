package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/unowned-22/api/internal/domain/event"
	domainstorage "github.com/unowned-22/api/internal/domain/storage"
	domainsys "github.com/unowned-22/api/internal/domain/systemsettings"
	domainusersettings "github.com/unowned-22/api/internal/domain/usersettings"
	"github.com/unowned-22/api/internal/logger"
)

// EmailVerifiedHandler processes user.email_verified events.
// It creates the per-user MinIO bucket and the initial user_settings row.
// This handler is intentionally separate from AuditHandler which consumes
// the audit.email_verified event — two different events, two different concerns.
type EmailVerifiedHandler struct {
	storage          domainstorage.Storage
	systemSettings   domainsys.Repository
	userSettingsRepo domainusersettings.Repository
}

// NewEmailVerifiedHandler creates a new provisioning handler for email verification.
func NewEmailVerifiedHandler(
	storage domainstorage.Storage,
	sysRepo domainsys.Repository,
	userSettingsRepo domainusersettings.Repository,
) *EmailVerifiedHandler {
	return &EmailVerifiedHandler{
		storage:          storage,
		systemSettings:   sysRepo,
		userSettingsRepo: userSettingsRepo,
	}
}

// EventName returns the event type this handler processes.
func (h *EmailVerifiedHandler) EventName() event.Name {
	return event.UserEmailVerified
}

// Handle provisions a bucket and user_settings for the newly verified user.
//
// Idempotency strategy:
//  1. If user_settings already exist the handler returns early — safe to retry.
//  2. BucketExists is called before CreateBucket so that a partial failure
//     (bucket created, user_settings insert failed) does not cause an error on
//     the next attempt from the DLQ.
func (h *EmailVerifiedHandler) Handle(ctx context.Context, payload []byte) error {
	var p struct {
		UserID int64 `json:"user_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("email_verified_handler: failed to unmarshal payload: %w", err)
	}
	if p.UserID == 0 {
		return fmt.Errorf("email_verified_handler: user_id is required in event payload")
	}

	// Idempotency guard: if user_settings already exist (e.g. handler retried
	// after a transient error) skip provisioning entirely.
	if existing, err := h.userSettingsRepo.GetByUserID(ctx, p.UserID); err == nil && existing != nil {
		logger.Log.WithFields(map[string]interface{}{
			"user_id": p.UserID,
		}).Info("email_verified_handler: user_settings already exist, skipping provisioning")
		return nil
	}

	// Read default storage quota from system settings.
	var quotaBytes int64
	if h.systemSettings != nil {
		if s, err := h.systemSettings.GetByKey(ctx, "default_storage_quota_bytes"); err == nil && s != nil {
			if err := json.Unmarshal(s.Value, &quotaBytes); err != nil {
				var f float64
				if err2 := json.Unmarshal(s.Value, &f); err2 == nil {
					quotaBytes = int64(f)
				}
			}
		}
	}

	bucketName := fmt.Sprintf("user-%d", p.UserID)

	if h.storage != nil {
		// Check existence first so retries after a partial failure don't error.
		exists, err := h.storage.BucketExists(ctx, bucketName)
		if err != nil {
			return fmt.Errorf("email_verified_handler: failed to check bucket existence for %s: %w", bucketName, err)
		}

		if !exists {
			if err := h.storage.CreateBucket(ctx, bucketName); err != nil {
				return fmt.Errorf("email_verified_handler: failed to create bucket %s: %w", bucketName, err)
			}
		}
	}

	// Persist initial user settings.
	if h.userSettingsRepo != nil {
		us := &domainusersettings.UserSettings{
			UserID:                  p.UserID,
			StorageQuotaBytes:       quotaBytes,
			StorageUsedBytes:        0,
			BucketName:              bucketName,
			Theme:                   json.RawMessage(`{}`),
			NotificationPreferences: json.RawMessage(`{}`),
		}
		if err := h.userSettingsRepo.Create(ctx, us); err != nil {
			return fmt.Errorf("email_verified_handler: failed to create user_settings: %w", err)
		}
	}

	logger.Log.WithFields(map[string]interface{}{
		"user_id": p.UserID,
		"bucket":  bucketName,
	}).Info("email_verified_handler: provisioned bucket and user_settings")

	return nil
}
