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
	// List returns a slice of users for pagination (offset,limit)
	List(ctx context.Context, offset int, limit int) ([]*User, error)
	// Count returns total number of users matching the query (currently all users)
	Count(ctx context.Context) (int64, error)
	SetVerificationToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error
	GetByVerificationToken(ctx context.Context, token string) (*User, error)
	MarkEmailVerified(ctx context.Context, userID int64) error
	UpdatePassword(ctx context.Context, userID int64, hashedPassword string) error
	SetDeactivatedAt(ctx context.Context, userID int64, t *time.Time) error
}

// UserService defines the application-level contract for user operations.
type UserService interface {
	GetProfile(ctx context.Context, userID int64) (*User, error)
	// ListUsers returns paginated users and the total count.
	ListUsers(ctx context.Context, page int, limit int) ([]*User, int64, error)
}
