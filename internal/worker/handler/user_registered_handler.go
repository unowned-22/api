package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/unowned-22/api/internal/domain/event"
	domainmailer "github.com/unowned-22/api/internal/domain/mailer"
	"github.com/unowned-22/api/internal/infrastructure/mailer"
	"github.com/unowned-22/api/internal/logger"
)

// UserRegisteredHandler processes user.registered events and sends a verification email.
type UserRegisteredHandler struct {
	mailer domainmailer.Mailer
}

// NewUserRegisteredHandler creates a new user registration event handler.
func NewUserRegisteredHandler(m domainmailer.Mailer) *UserRegisteredHandler {
	return &UserRegisteredHandler{
		mailer: m,
	}
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
	}

	if err := json.Unmarshal(payload, &eventPayload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	if eventPayload.Email == "" {
		return fmt.Errorf("email is required in event payload")
	}

	// Render verification email template
	htmlContent, textContent, err := mailer.RenderTemplate("verify_email", map[string]interface{}{
		"AppName":         "App",
		"VerificationURL": "https://app.local/verify", // TODO: Make dynamic
	})
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	// Send the email
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
