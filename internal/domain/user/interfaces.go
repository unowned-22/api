package user

import (
	"context"
	"time"
)

// UserRepository defines the persistence contract for users.
// Implementations live in internal/repository/postgres.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	SetVerificationToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error
	GetByVerificationToken(ctx context.Context, token string) (*User, error)
	MarkEmailVerified(ctx context.Context, userID int64) error
}

// UserService defines the application-level contract for user operations.
type UserService interface {
	GetProfile(ctx context.Context, userID int64) (*User, error)
}
