package token

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
// It holds a session FK and a rotation chain via ParentTokenID / ReplacedByTokenID.
type RefreshToken struct {
	ID                int64
	SessionID         int64
	UserID            int64
	TokenHash         string
	ParentTokenID     *int64
	ReplacedByTokenID *int64
	Status            RefreshTokenStatus
	CreatedAt         time.Time
	ExpiresAt         time.Time
}

// EffectiveStatus returns Expired when the token has passed its expiry time,
// regardless of the stored status field.
func (t *RefreshToken) EffectiveStatus() RefreshTokenStatus {
	if t.ExpiresAt.Before(time.Now()) {
		return RefreshTokenStatusExpired
	}
	return t.Status
}

// HashRefreshToken returns a SHA-256 hex string for the provided raw token.
func HashRefreshToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// RefreshTokenRepository defines the persistence contract for refresh tokens.
type RefreshTokenRepository interface {
	// Create inserts a new token and sets its ID via RETURNING.
	Create(ctx context.Context, token *RefreshToken) error
	// GetByHash retrieves a token by its pre-hashed value.
	GetByHash(ctx context.Context, tokenHash string) (*RefreshToken, error)
	// Rotate atomically revokes oldTokenID and inserts newToken in one transaction.
	// It sets replaced_by_token_id on the old token and parent_token_id on the new one.
	Rotate(ctx context.Context, oldTokenID int64, newToken *RefreshToken) error
	// Revoke marks a single token as revoked by ID.
	Revoke(ctx context.Context, tokenID int64) error
	// RevokeSessionTokens revokes all active tokens belonging to a session.
	RevokeSessionTokens(ctx context.Context, sessionID int64) error
	// RevokeAllByUserID revokes all tokens for a user (used on logout-all / deactivate).
	RevokeAllByUserID(ctx context.Context, userID int64) error
	// GetTokenChain returns all tokens for a session ordered by created_at ASC.
	GetTokenChain(ctx context.Context, sessionID int64) ([]*RefreshToken, error)
	// DeleteExpired removes all expired tokens from the database.
	DeleteExpired(ctx context.Context) error
}

// Manager is the primary auth contract used by services and middleware.
type Manager interface {
	Generate(userID int64) (string, error)
	Parse(token string) (int64, error)
}

// ManagerExtended is an optional extension of Manager used when role
// information must be embedded in or extracted from the access token.
type ManagerExtended interface {
	Manager
	GenerateWithRole(userID int64, tokenVersion int) (string, error)
	ParseWithRole(token string) (int64, string, int, error)
}
