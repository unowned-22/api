package dto

import "encoding/json"

type RegisterRequest struct {
	Email    string `json:"email"     validate:"required,email"`
	Password string `json:"password"  validate:"required,min=8"`
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
	Username string `json:"username"  validate:"required,min=3,max=30,username"`
	Phone    string `json:"phone"     validate:"omitempty,phone"`
}

type LoginRequest struct {
	Email      string `json:"email"       validate:"required,email"`
	Password   string `json:"password"    validate:"required"`
	DeviceName string `json:"device_name" validate:"omitempty,max=255"`
	OS         string `json:"os"          validate:"omitempty,max=100"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type ResendVerificationRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type PresignedUploadRequest struct {
	Filename    string `json:"filename"     validate:"required"`
	ContentType string `json:"content_type" validate:"required"`
}

type UpdateProfileRequest struct {
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
	Username string `json:"username"  validate:"required,min=3,max=30,username"`
	Phone    string `json:"phone"     validate:"omitempty,phone"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

type UpdateThemeRequest struct {
	Theme json.RawMessage `json:"theme" validate:"required"`
}

type CreateStoryRequest struct {
	Slides     []json.RawMessage `json:"slides"     validate:"required,min=1,max=20"`
	Visibility string            `json:"visibility" validate:"required,oneof=everyone friends close"`
	Duration   int               `json:"duration"   validate:"required,oneof=1 12 24 48"`
	HiddenFrom []int64           `json:"hidden_from"`
}
