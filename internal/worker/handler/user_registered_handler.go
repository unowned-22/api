package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unowned-22/api/internal/domain/event"
	domainmailer "github.com/unowned-22/api/internal/domain/mailer"
	"github.com/unowned-22/api/internal/infrastructure/mailer"
	"github.com/unowned-22/api/internal/logger"
)

// UserRegisteredHandler processes user.registered events and sends a verification email.
type UserRegisteredHandler struct {
	mailer  domainmailer.Mailer
	appURL  string
	appName string
}

// NewUserRegisteredHandler creates a new user registration event handler.
func NewUserRegisteredHandler(m domainmailer.Mailer, appURL, appName string) *UserRegisteredHandler {
	return &UserRegisteredHandler{mailer: m, appURL: appURL, appName: appName}
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
