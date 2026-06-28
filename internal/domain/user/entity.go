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
	CoverMobileURL             string
	CoverDesktopURL            string
	CreatedAt                  time.Time
	EmailVerifiedAt            *time.Time
	VerificationToken          *string
	VerificationTokenExpiresAt *time.Time
	DeactivatedAt              *time.Time
}

type UserCover struct {
	CoverURL        string
	CoverMobileURL  string
	CoverDesktopURL string
}

type CropRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type CoverCrop struct {
	Mobile  CropRect `json:"mobile"`
	Desktop CropRect `json:"desktop"`
}
