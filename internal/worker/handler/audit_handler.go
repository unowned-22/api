package handler

import (
	"context"
	"encoding/json"
	"fmt"

	domainAudit "github.com/unowned-22/api/internal/domain/audit"
	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/logger"
)

// AuditHandler writes audit events to the database.
type AuditHandler struct {
	repo      domainAudit.Repository
	eventName event.Name
}

// NewAuditHandler creates an AuditHandler for the given event name.
func NewAuditHandler(repo domainAudit.Repository, evt event.Name) *AuditHandler {
	return &AuditHandler{repo: repo, eventName: evt}
}

// EventName returns the event this handler listens to.
func (h *AuditHandler) EventName() event.Name {
	return h.eventName
}

// Handle parses a generic audit payload and persists it.
func (h *AuditHandler) Handle(ctx context.Context, payload []byte) error {
	var p map[string]interface{}
	if err := json.Unmarshal(payload, &p); err != nil {
		logger.Log.WithError(err).Warn("audit handler: failed to unmarshal payload")
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	var userID *int64
	if v, ok := p["user_id"]; ok {
		switch val := v.(type) {
		case float64:
			id := int64(val)
			userID = &id
		}
	}

	var ip *string
	if v, ok := p["ip_address"].(string); ok && v != "" {
		ip = &v
	}

	var ua *string
	if v, ok := p["user_agent"].(string); ok && v != "" {
		ua = &v
	}

	// Store entire payload as metadata
	meta := p

	entry := &domainAudit.AuditLog{
		UserID:    userID,
		EventType: string(h.eventName),
		IPAddress: ip,
		UserAgent: ua,
		Metadata:  meta,
	}

	if err := h.repo.Create(ctx, entry); err != nil {
		logger.Log.WithError(err).WithFields(map[string]interface{}{"event": h.eventName}).Error("failed to persist audit log")
		return fmt.Errorf("failed to persist audit log: %w", err)
	}

	return nil
}
