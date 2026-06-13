package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unowned-22/api/internal/domain/event"
	domainmailer "github.com/unowned-22/api/internal/domain/mailer"
	domainstorage "github.com/unowned-22/api/internal/domain/storage"
	domainsys "github.com/unowned-22/api/internal/domain/systemsettings"
	domainusersettings "github.com/unowned-22/api/internal/domain/usersettings"
	"github.com/unowned-22/api/internal/infrastructure/mailer"
	"github.com/unowned-22/api/internal/logger"
)

// UserRegisteredHandler processes user.registered events and sends a verification email.
type UserRegisteredHandler struct {
	mailer           domainmailer.Mailer
	appURL           string
	appName          string
	storage          domainstorage.Storage
	systemSettings   domainsys.Repository
	userSettingsRepo domainusersettings.Repository
}

// NewUserRegisteredHandler creates a new user registration event handler.
func NewUserRegisteredHandler(m domainmailer.Mailer, appURL, appName string, storage domainstorage.Storage, sysRepo domainsys.Repository, userSettingsRepo domainusersettings.Repository) *UserRegisteredHandler {
	return &UserRegisteredHandler{mailer: m, appURL: appURL, appName: appName, storage: storage, systemSettings: sysRepo, userSettingsRepo: userSettingsRepo}
}

// EventName returns the event type this handler processes.
func (h *UserRegisteredHandler) EventName() event.Name {
	return event.UserRegistered
}

// Handle processes a user.registered event by sending a verification email.
func (h *UserRegisteredHandler) Handle(ctx context.Context, payload []byte) error {
	var eventPayload struct {
		UserID int64  `json:"user_id"`
		Email  string `json:"email"`
		Token  string `json:"token"`
	}

	if err := json.Unmarshal(payload, &eventPayload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	if eventPayload.Email == "" || eventPayload.Token == "" {
		return fmt.Errorf("email and token are required in event payload")
	}

	// Create user storage bucket and default user settings using system settings
	// Load default storage quota
	var quotaBytes int64
	if h.systemSettings != nil {
		if s, err := h.systemSettings.GetByKey(ctx, "default_storage_quota_bytes"); err == nil && s != nil {
			if err := json.Unmarshal(s.Value, &quotaBytes); err != nil {
				// try number-as-float
				var f float64
				if err2 := json.Unmarshal(s.Value, &f); err2 == nil {
					quotaBytes = int64(f)
				}
			}
		}
	}

	// Determine bucket name and create it
	bucketName := fmt.Sprintf("user-%d", eventPayload.UserID)
	if h.storage != nil {
		if err := h.storage.CreateBucket(ctx, bucketName); err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
		}

		// apply bucket policy if available in system settings and storage supports it
		var policy string
		if h.systemSettings != nil {
			if p, err := h.systemSettings.GetByKey(ctx, "default_bucket_policy"); err == nil && p != nil {
				var pol string
				if err := json.Unmarshal(p.Value, &pol); err == nil {
					policy = pol
				}
			}
		}
		if policy != "" {
			// attempt type assertion to support optional policy application
			if applier, ok := h.storage.(interface {
				ApplyBucketPolicy(ctx context.Context, bucket, policy string) error
			}); ok {
				if err := applier.ApplyBucketPolicy(ctx, bucketName, policy); err != nil {
					return fmt.Errorf("failed to apply bucket policy: %w", err)
				}
			}
		}
	}

	// Persist initial user settings
	if h.userSettingsRepo != nil {
		us := &domainusersettings.UserSettings{
			UserID:            eventPayload.UserID,
			StorageQuotaBytes: quotaBytes,
			StorageUsedBytes:  0,
			BucketName:        bucketName,
			Theme:             json.RawMessage([]byte(`{}`)),
		}
		if err := h.userSettingsRepo.Create(ctx, us); err != nil {
			return fmt.Errorf("failed to create user settings: %w", err)
		}
	}

	verificationURL := strings.TrimRight(h.appURL, "/") + "/verify-email?token=" + eventPayload.Token

	htmlContent, textContent, err := mailer.RenderTemplate("verify_email", map[string]interface{}{
		"AppName":         h.appName,
		"VerificationURL": verificationURL,
	})
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	msg := domainmailer.Message{
		To:      []string{eventPayload.Email},
		Subject: "Verify Your Email Address",
		HTML:    htmlContent,
		Text:    textContent,
	}

	if err := h.mailer.Send(ctx, msg); err != nil {
		logger.Log.WithError(err).WithFields(map[string]interface{}{
			"event":   "user.registered",
			"email":   eventPayload.Email,
			"user_id": eventPayload.UserID,
		}).Error("Failed to send verification email")
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	return nil
}
