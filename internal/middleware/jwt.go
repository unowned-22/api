package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/token"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/logger"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// JWTAuth verifies the Authorization header, validates the JWT, and stores
// the user ID (and role when available) in the request context via contextx.
func JWTAuth(tokenManager token.Manager, userService user.UserService, cache user.TokenVersionCache) func(http.Handler) http.Handler {
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
			if ext, ok := tokenManager.(token.ManagerExtended); ok {
				userID, role, ver, err := ext.ParseWithRole(tokenStr)
				if err != nil {
					response.SendUnauthorized(w, "invalid or expired token")
					return
				}

				// Validate token version against cached or current user state
				var cachedVer int
				var cacheHit bool
				if cache != nil {
					if cv, found, err := cache.Get(r.Context(), userID); err == nil && found {
						cachedVer = cv
						cacheHit = true
					} else if err != nil {
						logger.Log.WithError(err).WithField("user_id", userID).Warn("token version cache Get failed")
					}
				}

				if !cacheHit {
					u, err := userService.GetProfile(r.Context(), userID)
					if err != nil {
						response.SendUnauthorized(w, "invalid token")
						return
					}

					// Verify account is active
					if u.DeactivatedAt != nil {
						response.SendUnauthorized(w, "user account is deactivated")
						return
					}

					cachedVer = u.TokenVersion
					if cache != nil {
						// Cache token version for 60 seconds (max delay for background invalidations)
						if err := cache.Set(r.Context(), userID, cachedVer, 60*time.Second); err != nil {
							logger.Log.WithError(err).WithField("user_id", userID).Warn("token version cache Set failed")
						}
					}
				}

				if cachedVer != ver {
					response.SendUnauthorized(w, "token version mismatch")
					return
				}

				ctx := contextx.SetUserID(r.Context(), userID)
				ctx = contextx.SetUserRole(ctx, role)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Fallback: base Manager only (no role in context).
			userID, err := tokenManager.Parse(tokenStr)
			if err != nil {
				response.SendUnauthorized(w, "invalid or expired token")
				return
			}
			ctx := contextx.SetUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
