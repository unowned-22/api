package user

import "time"

type User struct {
	ID                         int64
	Email                      string
	Password                   string
	TokenVersion               int
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
