package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/unowned-22/api/internal/domain/event"
	domainmailer "github.com/unowned-22/api/internal/domain/mailer"
	"github.com/unowned-22/api/internal/logger"
)

// EmailSendHandler processes email.send events and calls mailer.Send with retries.
type EmailSendHandler struct {
	mailer domainmailer.Mailer
	// max attempts
	attempts int
}

func NewEmailSendHandler(m domainmailer.Mailer) *EmailSendHandler {
	return &EmailSendHandler{mailer: m, attempts: 5}
}

func (h *EmailSendHandler) EventName() event.Name { return event.EmailSend }

func (h *EmailSendHandler) Handle(ctx context.Context, payload []byte) error {
	var p struct {
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		HTML    string   `json:"html"`
		Text    string   `json:"text"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		logger.Log.WithError(err).Warn("email handler: failed to unmarshal payload")
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	if len(p.To) == 0 {
		return fmt.Errorf("email.send: missing recipients")
	}

	msg := domainmailer.Message{To: p.To, Subject: p.Subject, HTML: p.HTML, Text: p.Text}

	var lastErr error
	for i := 1; i <= h.attempts; i++ {
		if err := h.mailer.Send(ctx, msg); err != nil {
			lastErr = err
			logger.Log.WithError(err).WithFields(map[string]interface{}{"attempt": i}).Warn("email handler: send failed, will retry")
			// exponential backoff with jitter simple sleep
			sleep := time.Duration(200*(1<<uint(i-1))) * time.Millisecond
			if sleep > 5*time.Second {
				sleep = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(sleep):
			}
			continue
		} else {
			return nil
		}
	}

	logger.Log.WithError(lastErr).Error("email handler: all attempts to send email failed")
	return fmt.Errorf("failed to send email after attempts: %w", lastErr)
}
