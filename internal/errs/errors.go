package errs

import "errors"

var (
	ErrUserNotFound              = errors.New("user not found")
	ErrInvalidCredentials        = errors.New("invalid credentials")
	ErrUserAlreadyExists         = errors.New("user already exists")
	ErrUsernameAlreadyExists     = errors.New("username already exists")
	ErrInvalidRefreshToken       = errors.New("refresh token is invalid")
	ErrRefreshTokenNotFound      = errors.New("refresh token not found")
	ErrRoleNotFound              = errors.New("role not found")
	ErrForbidden                 = errors.New("forbidden")
	ErrVerificationTokenInvalid  = errors.New("verification token is invalid or expired")
	ErrEmailAlreadyVerified      = errors.New("email already verified")
	ErrPasswordResetTokenInvalid = errors.New("password reset token is invalid or expired")
	ErrPasswordResetTokenUsed    = errors.New("password reset token has already been used")
	ErrEmailNotVerified          = errors.New("email not verified")
	ErrSessionNotFound           = errors.New("session not found")
	ErrUserDeactivated           = errors.New("user account is deactivated")

	// ErrUserStorageNotProvisioned is returned when the user's MinIO bucket or
	// user_settings row does not yet exist. This is a transient condition: the
	// email_verified worker creates the bucket asynchronously after verification,
	// so the provisioning may still be in flight (or may have failed and ended up
	// in the DLQ). The transport layer maps this to 503 Service Unavailable so
	// clients know to retry rather than treat the situation as a permanent error.
	ErrUserStorageNotProvisioned = errors.New("user storage is not yet provisioned")

	// ErrUserSettingsNotFound is returned when a user_settings row is expected
	// to exist but cannot be found. Semantically distinct from
	// ErrUserStorageNotProvisioned: use this when the lookup itself indicates a
	// missing record rather than an incomplete provisioning flow.
	ErrUserSettingsNotFound = errors.New("user settings not found")

	// ErrAvatarNotFound is returned when a user attempts to delete an avatar
	// that has not been uploaded.
	ErrAvatarNotFound = errors.New("avatar not found")

	// ErrCoverNotFound is returned when a user attempts to delete a cover
	// that has not been uploaded.
	ErrCoverNotFound = errors.New("cover not found")

	// Stories
	ErrStoryNotFound       = errors.New("story not found")
	ErrInvalidStoryPayload = errors.New("invalid story payload")

	// Device / session errors added for session-device refactor.
	ErrDeviceNotFound = errors.New("device not found")
	ErrSessionExpired = errors.New("session has expired")
	ErrSessionRevoked = errors.New("session has been revoked")
)
