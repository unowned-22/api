package handler

import (
	"net/http"

	domain "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/middleware"
	"github.com/unowned-22/api/internal/transport/http/response"
)

type AdminHandler struct {
	userService       domain.UserService
	permissionService domain.PermissionService
}

func NewAdminHandler(userService domain.UserService, permissionService domain.PermissionService) *AdminHandler {
	return &AdminHandler{
		userService:       userService,
		permissionService: permissionService,
	}
}

type AdminPingResponse struct {
	Message string `json:"message"`
}

// Ping returns a success message indicating admin access is verified
func (h *AdminHandler) Ping(w http.ResponseWriter, r *http.Request) {
	resp := AdminPingResponse{
		Message: "admin access granted",
	}
	response.SendSuccess(w, http.StatusOK, resp)
}

type PermissionResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

type AdminPermissionsResponse struct {
	Permissions []PermissionResponse `json:"permissions"`
}

func (h *AdminHandler) Permissions(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	user, err := h.userService.Profile(r.Context(), userID)
	if err != nil {
		response.SendError(w, err)
		return
	}

	permissions, err := h.permissionService.GetPermissionsByRole(r.Context(), user.RoleID)
	if err != nil {
		response.SendError(w, err)
		return
	}

	resp := AdminPermissionsResponse{
		Permissions: make([]PermissionResponse, 0, len(permissions)),
	}
	for _, permission := range permissions {
		resp.Permissions = append(resp.Permissions, PermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
			CreatedAt:   permission.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	response.SendSuccess(w, http.StatusOK, resp)
}
