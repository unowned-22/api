package handler

import (
	"encoding/json"
	"net/http"

	domain "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/transport/http/response"
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
	var req domain.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		response.SendBadRequest(w, "email and password are required")
		return
	}

	if err := h.userService.Register(r.Context(), req); err != nil {
		response.SendError(w, err)
		return
	}

	response.SendSuccess(w, http.StatusCreated, map[string]string{"message": "user registered successfully"})
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Login processes user authentication requests and returns access and refresh tokens
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req domain.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		response.SendBadRequest(w, "email and password are required")
		return
	}

	accessToken, refreshToken, err := h.userService.Login(r.Context(), req)
	if err != nil {
		response.SendError(w, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken string `json:"access_token"`
}

// Refresh issues a new access token using a valid refresh token
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		response.SendBadRequest(w, "refresh token is required")
		return
	}

	accessToken, err := h.userService.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		response.SendError(w, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, RefreshResponse{
		AccessToken: accessToken,
	})
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Logout revokes the given refresh token
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		response.SendBadRequest(w, "refresh token is required")
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
