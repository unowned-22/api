package usersession

import (
	"context"
	"time"
)

// UserSession represents a user device or client connection.
type UserSession struct {
	ID             int64
	UserID         int64
	RefreshTokenID int64
	DeviceName     string
	UserAgent      string
	IPAddress      string
	CreatedAt      time.Time
	LastUsedAt     time.Time
	RevokedAt      *time.Time
}

// UserSessionRepository defines the contract for persisting and managing sessions.
// Implementations live in the infrastructure/repository layer.
type UserSessionRepository interface {
	Create(ctx context.Context, session *UserSession) error
	GetByID(ctx context.Context, id int64) (*UserSession, error)
	GetByRefreshTokenID(ctx context.Context, refreshTokenID int64) (*UserSession, error)
	ListActiveByUserID(ctx context.Context, userID int64) ([]*UserSession, error)
	Update(ctx context.Context, session *UserSession) error
	Revoke(ctx context.Context, id int64) error
	RevokeAllByUserID(ctx context.Context, userID int64) error
}
