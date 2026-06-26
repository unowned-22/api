package response

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/logger"
)

type SuccessResponse struct {
	Data interface{} `json:"data"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// SendSuccess sends a JSON response with status 2xx and data wrapper
func SendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(SuccessResponse{Data: data})
}

// SendError maps application errors to standard HTTP response formats and writes to ResponseWriter
func SendError(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")

	var status int
	var code string
	var message string

	// Error contract reference:
	// - USER_NOT_FOUND -> 404
	// - USER_ALREADY_EXISTS -> 409
	// - USERNAME_ALREADY_EXISTS -> 409
	// - INVALID_CREDENTIALS -> 401
	// - INVALID_REFRESH_TOKEN -> 401
	// - INVALID_VERIFICATION_TOKEN -> 400
	// - EMAIL_ALREADY_VERIFIED -> 409
	// - ROLE_NOT_FOUND -> 404
	// - EMAIL_NOT_VERIFIED -> 403
	// - FORBIDDEN -> 403
	// - USER_SETTINGS_NOT_FOUND -> 404
	// - AVATAR_NOT_FOUND -> 404
	// - COVER_NOT_FOUND -> 404
	// - PHOTO_NOT_FOUND -> 404
	// - ALBUM_NOT_FOUND -> 404
	// - STORAGE_QUOTA_EXCEEDED -> 413
	// - PHOTO_ACCESS_DENIED / PHOTO_NOT_Owned / ALBUM_ACCESS_DENIED / ALBUM_NOT_OWNED -> 403
	// - STORY_NOT_FOUND -> 404
	// - COMMENT_NOT_FOUND -> 404
	// - COMMENT_NOT_OWNED -> 403
	// - COMMENT_NESTING_NOT_ALLOWED -> 422
	// - COMMENT_EDIT_EXPIRED -> 422
	// - COMMENT_ALREADY_DELETED -> 422
	// - ALREADY_LIKED -> 409
	// - NOT_LIKED -> 409
	// - INVALID_STORY_PAYLOAD -> 400
	// - STORAGE_NOT_PROVISIONED -> 503
	// - FRIENDSHIP_NOT_FOUND -> 404
	// - FRIENDSHIP_ALREADY_EXISTS -> 409
	// - CANNOT_FRIEND_YOURSELF -> 422
	// - NOT_ADDRESSEE -> 403
	// - NOT_REQUESTER -> 403
	// - NOT_FRIEND -> 404
	// - CLOSE_FRIEND_NOT_FOUND -> 404
	// - SESSION_EXPIRED -> 401
	// - SESSION_REVOKED -> 401
	// - SESSION_NOT_FOUND -> 404
	// - DEVICE_NOT_FOUND -> 404
	// - USER_DEACTIVATED -> 403
	// - INVALID_PASSWORD_RESET_TOKEN -> 400
	// - PASSWORD_RESET_TOKEN_USED -> 400

	switch {
	case errors.Is(err, errs.ErrUserNotFound):
		status = http.StatusNotFound
		code = "USER_NOT_FOUND"
		message = "user not found"

	case errors.Is(err, errs.ErrUserAlreadyExists):
		status = http.StatusConflict
		code = "USER_ALREADY_EXISTS"
		message = "user already exists"

	case errors.Is(err, errs.ErrUsernameAlreadyExists):
		status = http.StatusConflict
		code = "USERNAME_ALREADY_EXISTS"
		message = "this username is already taken"

	case errors.Is(err, errs.ErrInvalidCredentials):
		status = http.StatusUnauthorized
		code = "INVALID_CREDENTIALS"
		message = "invalid email or password"

	case errors.Is(err, errs.ErrInvalidRefreshToken), errors.Is(err, errs.ErrRefreshTokenNotFound):
		status = http.StatusUnauthorized
		code = "INVALID_REFRESH_TOKEN"
		message = "refresh token is invalid"

	case errors.Is(err, errs.ErrVerificationTokenInvalid):
		status = http.StatusBadRequest
		code = "INVALID_VERIFICATION_TOKEN"
		message = "verification token is invalid or expired"

	case errors.Is(err, errs.ErrEmailAlreadyVerified):
		status = http.StatusConflict
		code = "EMAIL_ALREADY_VERIFIED"
		message = "email already verified"

	case errors.Is(err, errs.ErrRoleNotFound):
		status = http.StatusNotFound
		code = "ROLE_NOT_FOUND"
		message = "role not found"

	case errors.Is(err, errs.ErrEmailNotVerified):
		status = http.StatusForbidden
		code = "EMAIL_NOT_VERIFIED"
		message = "please verify your email address before logging in"

	case errors.Is(err, errs.ErrForbidden):
		status = http.StatusForbidden
		code = "FORBIDDEN"
		message = "you do not have permission to access this resource"

	case errors.Is(err, errs.ErrUserBlocked):
		status = http.StatusForbidden
		code = "USER_BLOCKED"
		message = "you cannot send messages to this user"

	case errors.Is(err, errs.ErrFriendshipNotFound):
		status = http.StatusNotFound
		code = "FRIENDSHIP_NOT_FOUND"
		message = "friendship not found"

	case errors.Is(err, errs.ErrFriendshipAlreadyExist):
		status = http.StatusConflict
		code = "FRIENDSHIP_ALREADY_EXISTS"
		message = "friendship already exists"

	case errors.Is(err, errs.ErrCannotFriendYourself):
		status = http.StatusUnprocessableEntity
		code = "CANNOT_FRIEND_YOURSELF"
		message = "cannot send friendship request to yourself"

	case errors.Is(err, errs.ErrNotAddressee):
		status = http.StatusForbidden
		code = "NOT_ADDRESSEE"
		message = "only addressee can perform this action"

	case errors.Is(err, errs.ErrNotRequester):
		status = http.StatusForbidden
		code = "NOT_REQUESTER"
		message = "only requester can perform this action"

	case errors.Is(err, errs.ErrNotFriend):
		status = http.StatusNotFound
		code = "NOT_FRIEND"
		message = "users are not friends"

	case errors.Is(err, errs.ErrCloseFriendNotFound):
		status = http.StatusNotFound
		code = "CLOSE_FRIEND_NOT_FOUND"
		message = "close friend not found"

	case errors.Is(err, errs.ErrSessionExpired):
		status = http.StatusUnauthorized
		code = "SESSION_EXPIRED"
		message = "session has expired"

	case errors.Is(err, errs.ErrSessionRevoked):
		status = http.StatusUnauthorized
		code = "SESSION_REVOKED"
		message = "session has been revoked"

	case errors.Is(err, errs.ErrSessionNotFound):
		status = http.StatusNotFound
		code = "SESSION_NOT_FOUND"
		message = "session not found"

	case errors.Is(err, errs.ErrDeviceNotFound):
		status = http.StatusNotFound
		code = "DEVICE_NOT_FOUND"
		message = "device not found"

	case errors.Is(err, errs.ErrUserDeactivated):
		status = http.StatusForbidden
		code = "USER_DEACTIVATED"
		message = "user account is deactivated"

	case errors.Is(err, errs.ErrUserSettingsNotFound):
		status = http.StatusNotFound
		code = "USER_SETTINGS_NOT_FOUND"
		message = "user settings not found"

	case errors.Is(err, errs.ErrAvatarNotFound):
		status = http.StatusNotFound
		code = "AVATAR_NOT_FOUND"
		message = "avatar not found"

	case errors.Is(err, errs.ErrCoverNotFound):
		status = http.StatusNotFound
		code = "COVER_NOT_FOUND"
		message = "cover not found"

	case errors.Is(err, errs.ErrPhotoNotFound):
		status = http.StatusNotFound
		code = "PHOTO_NOT_FOUND"
		message = "photo not found"

	case errors.Is(err, errs.ErrAlbumNotFound):
		status = http.StatusNotFound
		code = "ALBUM_NOT_FOUND"
		message = "album not found"

	case errors.Is(err, errs.ErrStorageQuotaExceeded):
		status = http.StatusRequestEntityTooLarge
		code = "STORAGE_QUOTA_EXCEEDED"
		message = "storage quota exceeded"

	case errors.Is(err, errs.ErrPhotoAccessDenied), errors.Is(err, errs.ErrPhotoNotOwned), errors.Is(err, errs.ErrAlbumAccessDenied), errors.Is(err, errs.ErrAlbumNotOwned):
		status = http.StatusForbidden
		code = "FORBIDDEN"
		message = "forbidden"

	case errors.Is(err, errs.ErrStoryNotFound):
		status = http.StatusNotFound
		code = "STORY_NOT_FOUND"
		message = "story not found"

	case errors.Is(err, errs.ErrCommentNotFound):
		status = http.StatusNotFound
		code = "COMMENT_NOT_FOUND"
		message = "comment not found"

	case errors.Is(err, errs.ErrCommentNotOwned):
		status = http.StatusForbidden
		code = "FORBIDDEN"
		message = "forbidden"

	case errors.Is(err, errs.ErrCommentNestingNotAllowed):
		status = http.StatusUnprocessableEntity
		code = "COMMENT_NESTING_NOT_ALLOWED"
		message = "replies to replies are not allowed"

	case errors.Is(err, errs.ErrCommentEditExpired):
		status = http.StatusUnprocessableEntity
		code = "COMMENT_EDIT_EXPIRED"
		message = "comment edit window has expired"

	case errors.Is(err, errs.ErrCommentAlreadyDeleted):
		status = http.StatusUnprocessableEntity
		code = "COMMENT_ALREADY_DELETED"
		message = "comment is already deleted"

	case errors.Is(err, errs.ErrPasswordResetTokenInvalid):
		status = http.StatusBadRequest
		code = "INVALID_PASSWORD_RESET_TOKEN"
		message = "password reset token is invalid or expired"

	case errors.Is(err, errs.ErrPasswordResetTokenUsed):
		status = http.StatusBadRequest
		code = "PASSWORD_RESET_TOKEN_USED"
		message = "password reset token has already been used"

	case errors.Is(err, errs.ErrPhotoAlreadyLiked), errors.Is(err, errs.ErrCommentAlreadyLiked):
		status = http.StatusConflict
		code = "ALREADY_LIKED"
		message = "already liked"

	case errors.Is(err, errs.ErrPhotoNotLiked), errors.Is(err, errs.ErrCommentNotLiked):
		status = http.StatusConflict
		code = "NOT_LIKED"
		message = "not liked"

	case errors.Is(err, errs.ErrInvalidStoryPayload):
		status = http.StatusBadRequest
		code = "INVALID_STORY_PAYLOAD"
		message = "invalid story payload"

	// ErrUserStorageNotProvisioned is a transient state: the email_verified worker
	// provisions the bucket asynchronously. Return 503 so clients know to retry.
	case errors.Is(err, errs.ErrUserStorageNotProvisioned):
		status = http.StatusServiceUnavailable
		code = "STORAGE_NOT_PROVISIONED"
		message = "storage is not yet provisioned for this account, please try again shortly"

	default:
		logger.FromContext(r.Context()).WithError(err).Error("internal server error")
		status = http.StatusInternalServerError
		code = "INTERNAL_SERVER_ERROR"
		message = "internal server error"
	}

	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

func SendNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// SendBadRequest sends a custom 400 Bad Request error response
func SendBadRequest(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    "BAD_REQUEST",
			Message: message,
		},
	})
}

// SendUnauthorized sends a custom 401 Unauthorized error response
func SendUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    "UNAUTHORIZED",
			Message: message,
		},
	})
}

// SendTooManyRequests sends a 429 Too Many Requests error response
func SendTooManyRequests(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    "RATE_LIMITED",
			Message: message,
		},
	})
}

// SendForbidden sends a custom 403 Forbidden error response
func SendForbidden(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    "FORBIDDEN",
			Message: message,
		},
	})
}

// ValidationFieldError represents a single field that failed validation.
type ValidationFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrorDetail extends ErrorDetail with per-field validation details.
type ValidationErrorDetail struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details []ValidationFieldError `json:"details"`
}

// ValidationErrorResponse is the envelope for validation errors.
type ValidationErrorResponse struct {
	Error ValidationErrorDetail `json:"error"`
}

// SendValidationError sends a 422 Unprocessable Entity response with per-field validation details.
func SendValidationError(w http.ResponseWriter, fields []ValidationFieldError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(ValidationErrorResponse{
		Error: ValidationErrorDetail{
			Code:    "VALIDATION_ERROR",
			Message: "validation failed",
			Details: fields,
		},
	})
}

// SendNotFound sends a 404 Not Found error with a custom message.
func SendNotFound(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    "NOT_FOUND",
			Message: message,
		},
	})
}

// SendNotImplemented sends a 501 Not Implemented error with a custom message.
func SendNotImplemented(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    "NOT_IMPLEMENTED",
			Message: message,
		},
	})
}
