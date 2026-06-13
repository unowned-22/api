package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/domain/permission"
	"github.com/unowned-22/api/internal/domain/token"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/middleware"
	"github.com/unowned-22/api/internal/transport/http/handler"
	"golang.org/x/time/rate"
)

// NewRouter constructs the Chi router, registers middleware, and sets up all routes.
func NewRouter(
	cfg *config.Config,
	authHandler *handler.AuthHandler,
	userHandler *handler.UserHandler,
	passwordResetHandler *handler.PasswordResetHandler,
	adminHandler *handler.AdminHandler,
	uploadHandler *handler.UploadHandler,
	healthHandler *handler.HealthHandler,
	tokenManager token.Manager,
	userService user.UserService,
	permissionService permission.PermissionService,
) http.Handler {
	r := chi.NewRouter()

	// Global middleware stack.
	r.Use(middleware.CORS(cfg))
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recover)

	// Health check endpoints (no auth required).
	r.Get("/health/live", healthHandler.Live)
	r.Get("/health/ready", healthHandler.Ready)

	r.Route("/api/v1", func(r chi.Router) {
		// Global rate limiter for /api/v1
		r.Use(middleware.RateLimit(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst))

		// Public routes with stricter auth rate limiter.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RateLimit(rate.Limit(cfg.AuthRateLimitRPS), cfg.AuthRateLimitBurst))

			r.Post("/auth/register", authHandler.Register)
			r.Post("/auth/login", authHandler.Login)
			r.Get("/auth/verify-email", authHandler.VerifyEmail)
			r.Post("/auth/resend-verification", authHandler.ResendVerification)
			r.Post("/auth/forgot-password", passwordResetHandler.ForgotPassword)
			r.Post("/auth/reset-password", passwordResetHandler.ResetPassword)
			r.Post("/auth/refresh", authHandler.Refresh)
			r.Post("/auth/logout", authHandler.Logout)
		})

		// Authenticated routes.
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(tokenManager))

			r.Get("/users/me", userHandler.Me)
			r.Post("/uploads/presign", uploadHandler.Presign)

			// Role-gated: admin only.
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Get("/admin/ping", adminHandler.Ping)
			})

			// Permission-gated: requires admin.access.
			r.With(middleware.RequirePermission(permissionService, userService, "admin.access")).
				Get("/admin/permissions", adminHandler.Permissions)
		})
	})

	return r
}
