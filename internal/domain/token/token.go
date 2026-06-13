package token

import (
	"context"
	"time"
)

// RefreshTokenStatus defines the lifecycle states for refresh tokens.
type RefreshTokenStatus string

const (
	RefreshTokenStatusActive  RefreshTokenStatus = "active"
	RefreshTokenStatusRevoked RefreshTokenStatus = "revoked"
	RefreshTokenStatusExpired RefreshTokenStatus = "expired"
)

// RefreshToken is an opaque, revocable token that lets a client obtain a
// new access token without re-authenticating.
// UserID is a plain int64 scalar; importing domain/user is not required,
// which keeps the dependency graph acyclic.
type RefreshToken struct {
	ID        int64
	UserID    int64
	Token     string
	ExpiresAt time.Time
	Status    RefreshTokenStatus
	CreatedAt time.Time
}

func (t *RefreshToken) EffectiveStatus() RefreshTokenStatus {
	if t.ExpiresAt.Before(time.Now()) {
		return RefreshTokenStatusExpired
	}
	return t.Status
}

// RefreshTokenRepository defines the persistence contract for refresh tokens.
// Implementations live in internal/repository/postgres.
type RefreshTokenRepository interface {
	CreateRefreshToken(ctx context.Context, token *RefreshToken) error
	GetByToken(ctx context.Context, token string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, token string) error
	DeleteExpired(ctx context.Context) error
	RevokeAllByUserID(ctx context.Context, userID int64) error
}

// Manager is the primary auth contract used by services and middleware.
// It handles access token generation and parsing (user ID only).
// The public interface must remain stable.
type Manager interface {
	Generate(userID int64) (string, error)
	Parse(token string) (int64, error)
}

// ManagerExtended is an optional extension of Manager used when role
// information must be embedded in or extracted from the access token.
// JWTManager in internal/auth implements both interfaces.
// Services that need role-aware tokens accept this interface.
type ManagerExtended interface {
	Manager
	GenerateWithRole(userID int64, role string) (string, error)
	ParseWithRole(token string) (int64, string, error)
}
