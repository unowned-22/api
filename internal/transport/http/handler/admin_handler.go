package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/auth"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/permission"
	"github.com/unowned-22/api/internal/domain/systemsettings"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

// AdminHandler handles admin-scoped HTTP routes.
type AdminHandler struct {
	userService       user.UserService
	authService       auth.AuthService
	permissionService permission.PermissionService
	settingsService   systemsettings.Service
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(userService user.UserService, permissionService permission.PermissionService, authService auth.AuthService, settingsService systemsettings.Service) *AdminHandler {
	return &AdminHandler{
		userService:       userService,
		authService:       authService,
		permissionService: permissionService,
		settingsService:   settingsService,
	}
}

// Ping returns a success message confirming admin access.
func (h *AdminHandler) Ping(w http.ResponseWriter, r *http.Request) {
	response.SendSuccess(w, http.StatusOK, dto.AdminPingResponse{
		Message: "admin access granted",
	})
}

// Permissions lists all permissions held by the authenticated user's role.
func (h *AdminHandler) Permissions(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	u, err := h.userService.GetProfile(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	perms, err := h.permissionService.GetPermissionsByRole(r.Context(), u.RoleID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	resp := dto.AdminPermissionsResponse{
		Permissions: make([]dto.PermissionResponse, 0, len(perms)),
	}
	for _, p := range perms {
		resp.Permissions = append(resp.Permissions, dto.PermissionResponse{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	response.SendSuccess(w, http.StatusOK, resp)
}

// DeactivateUser deactivates the specified user account (admin only).
func (h *AdminHandler) DeactivateUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		response.SendBadRequest(w, "user id is required")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid user id")
		return
	}

	if err := h.authService.DeactivateUser(r.Context(), id); err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "user deactivated"})
}

// ReactivateUser re-enables a previously deactivated user account (admin only).
func (h *AdminHandler) ReactivateUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		response.SendBadRequest(w, "user id is required")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid user id")
		return
	}

	if err := h.authService.ReactivateUser(r.Context(), id); err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "user reactivated"})
}

// GetSettings returns all system settings (admin only).
func (h *AdminHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsService.GetAll(r.Context())
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, map[string]interface{}{"data": settings})
}

// UpdateSetting upserts a single system setting. Body: { "key": "theme", "value": { ... } }
func (h *AdminHandler) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key   string          `json:"key" validate:"required"`
		Value json.RawMessage `json:"value" validate:"required"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.SendValidationError(w, []response.ValidationFieldError{{Field: "key", Message: "required"}})
		return
	}

	if err := h.settingsService.Update(r.Context(), req.Key, req.Value); err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "setting updated"})
}
