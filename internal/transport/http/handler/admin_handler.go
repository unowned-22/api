package handler

import (
	"net/http"

	"github.com/unowned-22/api/internal/contextx"
	domain "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/transport/http/dto"
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

// Ping returns a success message indicating admin access is verified
func (h *AdminHandler) Ping(w http.ResponseWriter, r *http.Request) {
	resp := dto.AdminPingResponse{
		Message: "admin access granted",
	}
	response.SendSuccess(w, http.StatusOK, resp)
}

func (h *AdminHandler) Permissions(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	user, err := h.userService.GetProfile(r.Context(), userID)
	if err != nil {
		response.SendError(w, err)
		return
	}

	permissions, err := h.permissionService.GetPermissionsByRole(r.Context(), user.RoleID)
	if err != nil {
		response.SendError(w, err)
		return
	}

	resp := dto.AdminPermissionsResponse{
		Permissions: make([]dto.PermissionResponse, 0, len(permissions)),
	}
	for _, permission := range permissions {
		resp.Permissions = append(resp.Permissions, dto.PermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
			CreatedAt:   permission.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	response.SendSuccess(w, http.StatusOK, resp)
}
