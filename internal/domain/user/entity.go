package user

import "time"

// User is the core user entity.
// RoleID and RoleName are denormalised scalars; the canonical Role type
// lives in internal/domain/role to avoid a circular import.
type User struct {
	ID                         int64
	Email                      string
	Password                   string
	TokenVersion               int
	RoleID                     int64  // FK to roles.id
	RoleName                   string // denormalised for read convenience
	FullName                   string
	Username                   string
	Phone                      string
	AvatarURL                  string
	CoverURL                   string
	CreatedAt                  time.Time
	EmailVerifiedAt            *time.Time
	VerificationToken          *string
	VerificationTokenExpiresAt *time.Time
	DeactivatedAt              *time.Time
}
