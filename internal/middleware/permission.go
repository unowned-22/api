package middleware

import (
	"net/http"

	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/permission"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/logger"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// RequirePermission enforces that the authenticated user's role carries the
// named permission. Must be applied after JWTAuth.
func RequirePermission(
	permissionService permission.PermissionService,
	userService user.UserService,
	requiredPermission string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := contextx.UserID(r.Context())
			if !ok {
				response.SendUnauthorized(w, "unauthorized")
				return
			}

			u, err := userService.GetProfile(r.Context(), userID)
			if err != nil {
				logger.FromContext(r.Context()).WithError(err).Error("failed to check permissions")
				response.SendError(w, r, err)
				return
			}

			permissions, err := permissionService.GetPermissionsByRole(r.Context(), u.RoleID)
			if err != nil {
				logger.FromContext(r.Context()).WithError(err).Error("failed to check permissions")
				response.SendError(w, r, err)
				return
			}

			for _, p := range permissions {
				if p.Name == requiredPermission {
					next.ServeHTTP(w, r)
					return
				}
			}

			response.SendForbidden(w, "you do not have permission to access this resource")
		})
	}
}
