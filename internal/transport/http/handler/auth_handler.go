package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/unowned-22/api/internal/auth"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type AuthHandler struct {
	authService auth.AuthService
}

// NewAuthHandler creates a new instance of AuthHandler
func NewAuthHandler(authService auth.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
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

	authReq := auth.RegisterRequest{
		Email:    req.Email,
		Password: req.Password,
	}

	if err := h.authService.Register(r.Context(), authReq); err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusCreated, dto.MessageResponse{Message: "user registered successfully, please check your email to verify your account"})
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

	authReq := auth.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	}

	accessToken, refreshToken, err := h.authService.Login(r.Context(), authReq)
	if err != nil {
		response.SendError(w, r, err)
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

	accessToken, err := h.authService.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.AuthResponse{
		AccessToken: accessToken,
	})
}

// Logout revokes the given refresh token
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		response.SendBadRequest(w, "token is required")
		return
	}

	if err := h.authService.VerifyEmail(r.Context(), token); err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "email verified successfully"})
}

func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req dto.ResendVerificationRequest
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

	if err := h.authService.ResendVerification(r.Context(), req.Email); err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "verification email resent"})
}

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

	err := h.authService.Logout(r.Context(), req.RefreshToken)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "logged out successfully"})
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
