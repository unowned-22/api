package middleware

import (
	"net/http"

	"github.com/unowned-22/api/internal/domain/token"
	"github.com/unowned-22/api/internal/domain/user"
)

// WSJWTAuth is a thin wrapper around JWTAuth for WebSocket endpoints.
//
// Browsers cannot set the Authorization header on WebSocket handshake requests,
// so this middleware checks for a ?token=<jwt> query parameter as a fallback.
// If the Authorization header is already present it is used as-is, preserving
// full compatibility with regular HTTP clients.
//
// Usage in router:
//
//	r.With(middleware.WSJWTAuth(tokenManager, userService, tokenVersionCache)).
//	    Get("/ws/notifications", notificationHandler.Subscribe)
func WSJWTAuth(
	tokenManager token.Manager,
	userService user.UserService,
	cache user.TokenVersionCache,
) func(http.Handler) http.Handler {
	// Reuse the existing JWTAuth — all validation logic lives there.
	jwtMiddleware := JWTAuth(tokenManager, userService, cache)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only inject when the Authorization header is absent.
			if r.Header.Get("Authorization") == "" {
				if t := r.URL.Query().Get("token"); t != "" {
					// Clone so we don't mutate the original request headers.
					r = r.Clone(r.Context())
					r.Header.Set("Authorization", "Bearer "+t)
				}
			}
			jwtMiddleware(next).ServeHTTP(w, r)
		})
	}
}
