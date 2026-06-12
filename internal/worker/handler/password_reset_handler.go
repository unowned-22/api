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

// PasswordResetHandler processes password.reset.requested events and sends a reset email.
type PasswordResetHandler struct {
	mailer domainmailer.Mailer
}

// NewPasswordResetHandler creates a new password reset event handler.
func NewPasswordResetHandler(m domainmailer.Mailer) *PasswordResetHandler {
	return &PasswordResetHandler{
		mailer: m,
	}
}

// EventName returns the event type this handler processes.
func (h *PasswordResetHandler) EventName() event.Name {
	return event.PasswordResetRequested
}

// Handle processes a password.reset.requested event by sending a reset email.
func (h *PasswordResetHandler) Handle(ctx context.Context, payload []byte) error {
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

	// Render reset password email template
	htmlContent, textContent, err := mailer.RenderTemplate("reset_password", map[string]interface{}{
		"AppName":  "App",
		"ResetURL": "https://app.local/reset", // TODO: Make dynamic
	})
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	// Send the email
	msg := domainmailer.Message{
		To:      []string{eventPayload.Email},
		Subject: "Reset Your Password",
		HTML:    htmlContent,
		Text:    textContent,
	}

	if err := h.mailer.Send(ctx, msg); err != nil {
		logger.Log.WithError(err).WithFields(map[string]interface{}{
			"event":   "password.reset.requested",
			"email":   eventPayload.Email,
			"user_id": eventPayload.UserID,
		}).Error("Failed to send password reset email")
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	return nil
}
