package http

import (
	"bytes"
	"encoding/json"
	"io"
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

// emailExtractorFunc extracts email from request body for rate limiting.
func emailExtractorFunc(r *http.Request) string {
	if r.Body == nil {
		return ""
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}

	// Restore body for downstream handlers
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var payload struct {
		Email string `json:"email"`
	}

	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return ""
	}

	return payload.Email
}

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
	loginLimiter *middleware.AuthRateLimiter,
	registerLimiter *middleware.AuthRateLimiter,
	forgotPasswordLimiter *middleware.AuthRateLimiter,
	resendVerificationLimiter *middleware.AuthRateLimiter,
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

	// ── Swagger UI (development only) ─────────────────────────────────────────
	if handler.SwaggerAvailableInEnv(cfg.AppEnv) {
		swaggerHandler := handler.NewSwaggerHandler()

		r.Get("/swagger", swaggerHandler.Redirect)
		r.Get("/swagger/index.html", swaggerHandler.Index)
		r.Get("/swagger/openapi.yaml", swaggerHandler.Spec)
	}

	r.Route("/api/v1", func(r chi.Router) {
		// Global rate limiter for /api/v1
		r.Use(middleware.RateLimit(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst))

		// Public auth routes with endpoint-specific rate limiters.
		r.Group(func(r chi.Router) {
			// Register endpoint - rate limit by IP and email
			r.With(middleware.AuthRateLimitByEmail("register", registerLimiter, emailExtractorFunc)).
				Post("/auth/register", authHandler.Register)

			// Login endpoint - rate limit by IP and email
			r.With(middleware.AuthRateLimitByEmail("login", loginLimiter, emailExtractorFunc)).
				Post("/auth/login", authHandler.Login)

			// Email verification - rate limit by IP only
			r.With(middleware.AuthRateLimitByIP("verify-email", registerLimiter)).
				Get("/auth/verify-email", authHandler.VerifyEmail)

			// Resend verification - rate limit by IP and email
			r.With(middleware.AuthRateLimitByEmail("resend-verification", resendVerificationLimiter, emailExtractorFunc)).
				Post("/auth/resend-verification", authHandler.ResendVerification)

			// Forgot password - rate limit by IP and email
			r.With(middleware.AuthRateLimitByEmail("forgot-password", forgotPasswordLimiter, emailExtractorFunc)).
				Post("/auth/forgot-password", passwordResetHandler.ForgotPassword)

			// Reset password - rate limit by IP only
			r.With(middleware.AuthRateLimitByIP("reset-password", forgotPasswordLimiter)).
				Post("/auth/reset-password", passwordResetHandler.ResetPassword)

			// Refresh and logout - use generic rate limiter
			r.Use(middleware.RateLimit(rate.Limit(cfg.AuthRateLimitRPS), cfg.AuthRateLimitBurst))
			r.Post("/auth/refresh", authHandler.Refresh)
			r.Post("/auth/logout", authHandler.Logout)
		})

		// Authenticated routes.
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(tokenManager))

			r.Get("/users/me", userHandler.Me)
			r.Patch("/users/me", userHandler.UpdateProfile)
			// List users (requires users.read permission)
			r.With(middleware.RequirePermission(permissionService, userService, "users.read")).Get("/users", userHandler.List)
			r.Get("/auth/sessions", authHandler.ListSessions)
			r.Delete("/auth/sessions/{id}", authHandler.RevokeSession)
			r.Post("/auth/logout-all", authHandler.LogoutAll)
			r.Post("/uploads/presign", uploadHandler.Presign)

			// Role-gated: admin only.
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Get("/admin/ping", adminHandler.Ping)
				r.Post("/admin/users/{id}/deactivate", adminHandler.DeactivateUser)
				r.Post("/admin/users/{id}/reactivate", adminHandler.ReactivateUser)
			})

			// Permission-gated: requires admin.access.
			r.With(middleware.RequirePermission(permissionService, userService, "admin.access")).
				Get("/admin/permissions", adminHandler.Permissions)
		})
	})

	return r
}
