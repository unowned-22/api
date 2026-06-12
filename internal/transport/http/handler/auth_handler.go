package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	domain "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type AuthHandler struct {
	userService domain.UserService
}

// NewAuthHandler creates a new instance of AuthHandler
func NewAuthHandler(userService domain.UserService) *AuthHandler {
	return &AuthHandler{userService: userService}
}

// Register processes user registration requests
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		if ve, ok := errors.AsType[*validator.ValidationErrors](err); ok {
			response.SendValidationError(w, toFieldErrors(ve.Fields))
			return
		}

		response.SendBadRequest(w, "invalid request")
		return
	}

	if err := h.userService.Register(r.Context(), req.Email, req.Password); err != nil {
		response.SendError(w, err)
		return
	}

	response.SendSuccess(w, http.StatusCreated, map[string]string{"message": "user registered successfully"})
}

// Login processes user authentication requests and returns access and refresh tokens
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		if ve, ok := errors.AsType[*validator.ValidationErrors](err); ok {
			response.SendValidationError(w, toFieldErrors(ve.Fields))
			return
		}
		response.SendBadRequest(w, "invalid request")
		return
	}

	accessToken, refreshToken, err := h.userService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		response.SendError(w, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// Refresh issues a new access token using a valid refresh token
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		if ve, ok := errors.AsType[*validator.ValidationErrors](err); ok {
			response.SendValidationError(w, toFieldErrors(ve.Fields))
			return
		}
		response.SendBadRequest(w, "invalid request")
		return
	}

	accessToken, err := h.userService.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		response.SendError(w, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.AuthResponse{
		AccessToken: accessToken,
	})
}

// Logout revokes the given refresh token
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req dto.LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		if ve, ok := errors.AsType[*validator.ValidationErrors](err); ok {
			response.SendValidationError(w, toFieldErrors(ve.Fields))
			return
		}
		response.SendBadRequest(w, "invalid request")
		return
	}

	err := h.userService.Logout(r.Context(), req.RefreshToken)
	if err != nil {
		response.SendError(w, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, map[string]string{
		"message": "logged out successfully",
	})
}

// toFieldErrors converts validator field errors to response field errors,
// keeping the validator package independent of the response package.
func toFieldErrors(fields []validator.FieldError) []response.ValidationFieldError {
	out := make([]response.ValidationFieldError, 0, len(fields))
	for _, f := range fields {
		out = append(out, response.ValidationFieldError{
			Field:   f.Field,
			Message: f.Message,
		})
	}
	return out
}
