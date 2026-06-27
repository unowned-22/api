package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/domain/usersettings"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

// UserHandler handles user-scoped HTTP routes.
type UserHandler struct {
	userService     user.UserService
	settingsService usersettings.Service
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(userService user.UserService, settingsService usersettings.Service) *UserHandler {
	return &UserHandler{userService: userService, settingsService: settingsService}
}

// Me returns the profile of the currently authenticated user.
func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	h.getMe(userID, w, r)
}

// UpdateProfile updates the profile of the currently authenticated user.
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	var req dto.UpdateProfileRequest
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

	if err := h.userService.UpdateProfile(r.Context(), userID, req.FullName, req.Username, req.Phone); err != nil {
		response.SendError(w, r, err)
		return
	}

	h.getMe(userID, w, r)
}

func (h *UserHandler) GetMySettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	s, err := h.settingsService.GetMySettings(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.UserSettingsResponse{
		UserID:            s.UserID,
		StorageQuotaBytes: s.StorageQuotaBytes,
		StorageUsedBytes:  s.StorageUsedBytes,
		BucketName:        s.BucketName,
		Theme:             s.Theme,
		UpdatedAt:         s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// UpdateMySettings allows the user to update their theme.
func (h *UserHandler) UpdateMySettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	var req dto.UpdateThemeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		response.SendValidationError(w, []response.ValidationFieldError{{Field: "theme", Message: "invalid"}})
		return
	}

	if err := h.settingsService.UpdateMyTheme(r.Context(), userID, req.Theme); err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "theme updated"})
}

// UpdateMyNotificationPreferences updates the authenticated user's notification preferences.
func (h *UserHandler) UpdateMyNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	if err := h.settingsService.UpdateMyNotificationPreferences(r.Context(), userID, raw); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "notification preferences updated"})
}

func (h *UserHandler) getMe(userID int64, w http.ResponseWriter, r *http.Request) {
	u, err := h.userService.GetProfile(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		FullName:  u.FullName,
		Username:  u.Username,
		Phone:     u.Phone,
		AvatarURL: u.AvatarURL,
		CoverURL:  u.CoverURL,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}
