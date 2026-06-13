package dto

import "encoding/json"

// AuthResponse is the HTTP response body for login and token refresh.
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// UserResponse is the HTTP response body for user profile endpoints.
type UserResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FullName  string `json:"full_name"`
	Username  string `json:"username"`
	Phone     string `json:"phone"`
	AvatarURL string `json:"avatar_url"`
	CoverURL  string `json:"cover_url"`
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

type SessionResponse struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	DeviceName string `json:"device_name"`
	UserAgent  string `json:"user_agent"`
	IPAddress  string `json:"ip_address"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at"`
}

type SessionListResponse struct {
	Sessions []SessionResponse `json:"sessions"`
}

type UserSettingsResponse struct {
	UserID            int64           `json:"user_id"`
	StorageQuotaBytes int64           `json:"storage_quota_bytes"`
	StorageUsedBytes  int64           `json:"storage_used_bytes"`
	BucketName        string          `json:"bucket_name"`
	Theme             json.RawMessage `json:"theme"`
	UpdatedAt         string          `json:"updated_at"`
}
