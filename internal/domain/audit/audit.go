package audit

import (
	"context"
	"time"
)

// AuditLog represents a security/audit record.
type AuditLog struct {
	ID        int64
	UserID    *int64
	EventType string
	IPAddress *string
	UserAgent *string
	Metadata  map[string]interface{}
	CreatedAt time.Time
}

// Repository is the persistence contract for audit logs.
type Repository interface {
	Create(ctx context.Context, a *AuditLog) error
}
