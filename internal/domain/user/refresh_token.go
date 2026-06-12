package user

import (
	"context"
	"time"
)

type RefreshToken struct {
	ID        int64
	UserID    int64
	Token     string
	ExpiresAt time.Time
	Revoked   bool
	CreatedAt time.Time
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *RefreshToken) error
	GetByToken(ctx context.Context, token string) (*RefreshToken, error)
	Revoke(ctx context.Context, token string) error
	DeleteExpired(ctx context.Context) error
}
