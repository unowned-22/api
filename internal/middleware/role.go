package middleware

import (
	"net/http"

	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// RequireRole returns a middleware that allows only users whose JWT role claim
// matches the required role. Must be applied after JWTAuth.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole, ok := contextx.UserRole(r.Context())
			if !ok || userRole != role {
				response.SendForbidden(w, "you do not have permission to access this resource")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
