package dto

// RegisterRequest is the HTTP request body for user registration.
type RegisterRequest struct {
	Email    string `json:"email"     validate:"required,email"`
	Password string `json:"password"  validate:"required,min=8"`
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
	Username string `json:"username"  validate:"required,min=3,max=30,username"`
	Phone    string `json:"phone"     validate:"omitempty,phone"`
}

// LoginRequest is the HTTP request body for user authentication.
type LoginRequest struct {
	Email      string `json:"email"       validate:"required,email"`
	Password   string `json:"password"    validate:"required"`
	DeviceName string `json:"device_name" validate:"omitempty,max=255"`
}

// RefreshRequest is the HTTP request body for token refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutRequest is the HTTP request body for user logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ResendVerificationRequest is the HTTP request body for resending verification emails.
type ResendVerificationRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ForgotPasswordRequest is the HTTP request body for password reset initiation.
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordRequest is the HTTP request body for resetting a password.
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// PresignedUploadRequest содержит метаданные для генерации presigned URL.
type PresignedUploadRequest struct {
	Filename    string `json:"filename"     validate:"required"`
	ContentType string `json:"content_type" validate:"required"`
}

// UpdateProfileRequest is the HTTP request body for updating the current user's profile.
type UpdateProfileRequest struct {
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
	Username string `json:"username"  validate:"required,min=3,max=30,username"`
	Phone    string `json:"phone"     validate:"omitempty,phone"`
}
