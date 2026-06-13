package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/auth"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/permission"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// AdminHandler handles admin-scoped HTTP routes.
type AdminHandler struct {
	userService       user.UserService
	authService       auth.AuthService
	permissionService permission.PermissionService
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(userService user.UserService, permissionService permission.PermissionService, authService auth.AuthService) *AdminHandler {
	return &AdminHandler{
		userService:       userService,
		authService:       authService,
		permissionService: permissionService,
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
