package middleware

import (
	"context"
	"net/http"
	"strings"

	domain "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/transport/http/response"
)

type userContextKey string

const (
	UserIDContextKey   userContextKey = "user_id"
	UserRoleContextKey userContextKey = "user_role"
)

// JWTAuth verifies the Authorization header, validates the JWT,
// and stores user_id (and role if available) in the request context.
func JWTAuth(tokenManager domain.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.SendUnauthorized(w, "missing authorization header")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				response.SendUnauthorized(w, "invalid authorization header format")
				return
			}

			tokenStr := parts[1]

			// Use the extended interface when available to extract role alongside user ID.
			if ext, ok := tokenManager.(domain.TokenManagerExtended); ok {
				userID, role, err := ext.ParseWithRole(tokenStr)
				if err != nil {
					response.SendUnauthorized(w, "invalid or expired token")
					return
				}
				ctx := context.WithValue(r.Context(), UserIDContextKey, userID)
				ctx = context.WithValue(ctx, UserRoleContextKey, role)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Fallback: base TokenManager only (no role in context).
			userID, err := tokenManager.Parse(tokenStr)
			if err != nil {
				response.SendUnauthorized(w, "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), UserIDContextKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID retrieves the authenticated user ID from the request context.
func GetUserID(ctx context.Context) (int64, bool) {
	val, ok := ctx.Value(UserIDContextKey).(int64)
	return val, ok
}

// GetUserRole retrieves the authenticated user's role from the request context.
func GetUserRole(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(UserRoleContextKey).(string)
	return val, ok
}
