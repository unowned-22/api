package passwordreset

import (
	"context"
	"time"
)

type Token struct {
	ID        int64
	UserID    int64
	Token     string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type Repository interface {
	Create(ctx context.Context, t *Token) error
	GetByToken(ctx context.Context, token string) (*Token, error)
	MarkUsed(ctx context.Context, token string) error
	DeleteByUserID(ctx context.Context, userID int64) error
}
