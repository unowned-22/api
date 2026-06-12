package middleware

import (
	"net/http"

	"github.com/unowned-22/api/internal/contextx"
	domain "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/transport/http/response"
)

func RequirePermission(
	permissionService domain.PermissionService,
	userService domain.UserService,
	requiredPermission string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := contextx.UserID(r.Context())
			if !ok {
				response.SendUnauthorized(w, "unauthorized")
				return
			}

			user, err := userService.GetProfile(r.Context(), userID)
			if err != nil {
				response.SendError(w, err)
				return
			}

			permissions, err := permissionService.GetPermissionsByRole(r.Context(), user.RoleID)
			if err != nil {
				response.SendError(w, err)
				return
			}

			for _, permission := range permissions {
				if permission.Name == requiredPermission {
					next.ServeHTTP(w, r)
					return
				}
			}

			response.SendForbidden(w, "you do not have permission to access this resource")
		})
	}
}
