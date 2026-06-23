package user

import (
	"context"
	"io"
	"time"
)

// UserRepository defines the persistence contract for users.
// Implementations live in internal/repository/postgres.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	// GetByUsername returns a user by their unique username.
	GetByUsername(ctx context.Context, username string) (*User, error)
	// List returns a slice of users for pagination (offset,limit)
	List(ctx context.Context, offset int, limit int) ([]*User, error)
	// Count returns total number of users matching the query (currently all users)
	Count(ctx context.Context) (int64, error)
	SetVerificationToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error
	GetByVerificationToken(ctx context.Context, token string) (*User, error)
	MarkEmailVerified(ctx context.Context, userID int64) error
	UpdatePassword(ctx context.Context, userID int64, hashedPassword string) error
	IncrementTokenVersion(ctx context.Context, userID int64) error
	SetDeactivatedAt(ctx context.Context, userID int64, t *time.Time) error
	UpdateProfile(ctx context.Context, userID int64, fullName, username, phone string) error
	UpdateAvatar(ctx context.Context, userID int64, avatarURL string) error
	UpdateCover(ctx context.Context, userID int64, coverURL string) error
}

// UserService defines the application-level contract for user operations.
type UserService interface {
	GetProfile(ctx context.Context, userID int64) (*User, error)
	// ListUsers returns paginated users and the total count.
	ListUsers(ctx context.Context, page int, limit int) ([]*User, int64, error)
	UpdateProfile(ctx context.Context, userID int64, fullName, username, phone string) error
	UploadAvatar(ctx context.Context, userID int64, file io.Reader, size int64, contentType string) (string, error)
	UploadCover(ctx context.Context, userID int64, file io.Reader, size int64, contentType string) (string, error)
	DeleteAvatar(ctx context.Context, userID int64) error
	DeleteCover(ctx context.Context, userID int64) error
}

// TokenVersionCache defines the caching contract for user token versions.
type TokenVersionCache interface {
	Get(ctx context.Context, userID int64) (int, bool, error)
	Set(ctx context.Context, userID int64, version int, ttl time.Duration) error
	Delete(ctx context.Context, userID int64) error
}
