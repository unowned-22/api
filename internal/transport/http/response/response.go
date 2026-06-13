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

	if errors.Is(err, errs.ErrUserNotFound) {
		status = http.StatusNotFound
		code = "USER_NOT_FOUND"
		message = "user not found"
	} else if errors.Is(err, errs.ErrUserAlreadyExists) {
		status = http.StatusConflict
		code = "USER_ALREADY_EXISTS"
		message = "user already exists"
	} else if errors.Is(err, errs.ErrUsernameAlreadyExists) {
		status = http.StatusConflict
		code = "USERNAME_ALREADY_EXISTS"
		message = "this username is already taken"
	} else if errors.Is(err, errs.ErrInvalidCredentials) {
		status = http.StatusUnauthorized
		code = "INVALID_CREDENTIALS"
		message = "invalid email or password"
	} else if errors.Is(err, errs.ErrInvalidRefreshToken) || errors.Is(err, errs.ErrRefreshTokenNotFound) {
		status = http.StatusUnauthorized
		code = "INVALID_REFRESH_TOKEN"
		message = "refresh token is invalid"
	} else if errors.Is(err, errs.ErrVerificationTokenInvalid) {
		status = http.StatusBadRequest
		code = "INVALID_VERIFICATION_TOKEN"
		message = "verification token is invalid or expired"
	} else if errors.Is(err, errs.ErrEmailAlreadyVerified) {
		status = http.StatusConflict
		code = "EMAIL_ALREADY_VERIFIED"
		message = "email already verified"
	} else if errors.Is(err, errs.ErrRoleNotFound) {
		status = http.StatusInternalServerError
		code = "INTERNAL_SERVER_ERROR"
		message = "internal server error"
	} else if errors.Is(err, errs.ErrEmailNotVerified) {
		status = http.StatusForbidden
		code = "EMAIL_NOT_VERIFIED"
		message = "please verify your email address before logging in"
	} else if errors.Is(err, errs.ErrForbidden) {
		status = http.StatusForbidden
		code = "FORBIDDEN"
		message = "you do not have permission to access this resource"
	} else {
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
