package usersession

import (
	"context"
	"time"
)

// SessionStatus defines the lifecycle states for sessions.
type SessionStatus string

const (
	SessionStatusActive  SessionStatus = "active"
	SessionStatusRevoked SessionStatus = "revoked"
	SessionStatusExpired SessionStatus = "expired"
)

// UserSession represents a stable session tied to a device.
// It is never mutated on token rotation — the refresh token chain owns that.
type UserSession struct {
	ID             int64
	UserID         int64
	DeviceID       *int64 // nullable: existing sessions may not have a device
	Status         SessionStatus
	CreatedAt      time.Time
	LastActivityAt time.Time
	ExpiresAt      time.Time
}

// SessionView extends UserSession with denormalised device fields for list queries.
type SessionView struct {
	UserSession
	DeviceName string
	Browser    string
	OS         string
}

// UserSessionRepository defines the contract for persisting and managing sessions.
type UserSessionRepository interface {
	Create(ctx context.Context, session *UserSession) error
	GetByID(ctx context.Context, id int64) (*UserSession, error)
	ListActiveByUserID(ctx context.Context, userID int64) ([]*SessionView, error)
	Terminate(ctx context.Context, id int64) error
	TerminateAll(ctx context.Context, userID int64) error
	UpdateLastActivity(ctx context.Context, id int64, t time.Time) error
}
