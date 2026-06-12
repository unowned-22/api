package dto

// RegisterRequest is the HTTP request body for user registration.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest is the HTTP request body for user authentication.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshRequest is the HTTP request body for token refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// LogoutRequest is the HTTP request body for user logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}
