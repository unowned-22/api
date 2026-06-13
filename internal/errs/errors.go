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
)
