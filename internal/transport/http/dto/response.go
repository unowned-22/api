package dto

// AuthResponse is the HTTP response body for login and token refresh.
// RefreshToken is omitted in refresh responses (only access token is reissued).
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// UserResponse is the HTTP response body for user profile endpoints.
type UserResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

// AdminPingResponse is the HTTP response body for the admin ping endpoint.
type AdminPingResponse struct {
	Message string `json:"message"`
}

// PermissionResponse represents a single permission in HTTP responses.
type PermissionResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

// AdminPermissionsResponse is the HTTP response body for the admin permissions endpoint.
type AdminPermissionsResponse struct {
	Permissions []PermissionResponse `json:"permissions"`
}

type PresignedUploadResponse struct {
	UploadURL string `json:"upload_url"`
	Key       string `json:"key"`
	ExpiresIn int    `json:"expires_in"`
}

type MessageResponse struct {
	Message string `json:"message"`
}
