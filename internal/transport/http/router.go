package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	domain "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/middleware"
	"github.com/unowned-22/api/internal/transport/http/handler"
)

// NewRouter constructs the Chi router, registers middlewares, and sets up routes
func NewRouter(
	authHandler *handler.AuthHandler,
	userHandler *handler.UserHandler,
	adminHandler *handler.AdminHandler,
	tokenManager domain.TokenManager,
	userService domain.UserService,
	permissionService domain.PermissionService,
) http.Handler {
	r := chi.NewRouter()

	// Register global middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recover)

	// Register routes under version prefix
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Post("/auth/register", authHandler.Register)
		r.Post("/auth/login", authHandler.Login)
		r.Post("/auth/refresh", authHandler.Refresh)
		r.Post("/auth/logout", authHandler.Logout)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(tokenManager))

			r.Get("/users/me", userHandler.Me)

			// Admin only routes
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Get("/admin/ping", adminHandler.Ping)
			})

			r.With(middleware.RequirePermission(permissionService, userService, "admin.access")).
				Get("/admin/permissions", adminHandler.Permissions)
		})
	})

	return r
}
