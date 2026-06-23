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
	storyHandler *handler.StoryHandler,
	friendshipHandler *handler.FriendshipHandler,
	profileHandler *handler.ProfileHandler,
	notificationHandler *handler.NotificationHandler,
	tokenManager token.Manager,
	userService user.UserService,
	permissionService permission.PermissionService,
	loginLimiter *middleware.AuthRateLimiter,
	registerLimiter *middleware.AuthRateLimiter,
	forgotPasswordLimiter *middleware.AuthRateLimiter,
	resendVerificationLimiter *middleware.AuthRateLimiter,
	tokenVersionCache user.TokenVersionCache,
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
		r.Group(func(r chi.Router) {
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

		r.With(middleware.WSJWTAuth(tokenManager, userService, tokenVersionCache)).
			Get("/ws/notifications", notificationHandler.Subscribe)

		// Authenticated routes.
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(tokenManager, userService, tokenVersionCache))

			r.Get("/users/me", userHandler.Me)
			r.Patch("/users/me", userHandler.UpdateProfile)
			r.Put("/users/me/password", authHandler.ChangePassword)
			r.Get("/users/me/settings", userHandler.GetMySettings)
			r.Patch("/users/me/settings", userHandler.UpdateMySettings)
			r.Patch("/users/me/settings/notifications", userHandler.UpdateMyNotificationPreferences)
			// List users (requires users.read permission)
			r.With(middleware.RequirePermission(permissionService, userService, "users.read")).Get("/users", userHandler.List)
			r.Get("/auth/sessions", authHandler.ListSessions)
			r.Delete("/auth/sessions/{id}", authHandler.RevokeSession)
			r.Post("/auth/logout-all", authHandler.LogoutAll)
			r.Post("/uploads/presign", uploadHandler.Presign)
			r.Post("/users/me/avatar", uploadHandler.UploadAvatar)
			r.Post("/users/me/cover", uploadHandler.UploadCover)
			r.Delete("/users/me/avatar", uploadHandler.DeleteAvatar)
			r.Delete("/users/me/cover", uploadHandler.DeleteCover)
			r.With(middleware.RateLimit(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst)).Post("/stories", storyHandler.Publish)
			r.Get("/stories/me", storyHandler.ListMine)
			r.Get("/stories/feed", storyHandler.Feed)
			r.Delete("/stories/{id}", storyHandler.Delete)
			r.Post("/stories/{id}/view", storyHandler.View)
			r.Post("/stories/{id}/like", storyHandler.Like)
			r.Post("/stories/{id}/unlike", storyHandler.Unlike)
			r.Post("/stories/{id}/reply", storyHandler.Reply)
			r.With(middleware.RateLimit(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst)).Post("/stories/media", uploadHandler.UploadStoryMedia)

			// Notifications
			r.Get("/notifications", notificationHandler.List)
			r.Get("/notifications/unread-count", notificationHandler.UnreadCount)
			r.Post("/notifications/{id}/read", notificationHandler.MarkRead)
			r.Post("/notifications/read-all", notificationHandler.MarkAllRead)

			// Friends / subscriptions
			r.Post("/friends/requests", friendshipHandler.SendRequest)
			r.Post("/friends/requests/{id}/accept", friendshipHandler.Accept)
			r.Post("/friends/requests/{id}/reject", friendshipHandler.Reject)
			r.Post("/friends/requests/{id}/cancel", friendshipHandler.Cancel)
			r.Get("/friends/requests/incoming", friendshipHandler.ListIncoming)
			r.Get("/friends/requests/outgoing", friendshipHandler.ListOutgoing)
			r.Get("/friends/suggestions", friendshipHandler.ListSuggestions)
			r.Get("/friends", friendshipHandler.ListFriends)
			r.Delete("/friends/{id}", friendshipHandler.Remove)

			// Public profile by username
			r.Get("/users/{username}", profileHandler.GetByUsername)

			// Role-gated: admin only.
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Get("/admin/ping", adminHandler.Ping)
				r.Get("/admin/settings", adminHandler.GetSettings)
				r.Patch("/admin/settings", adminHandler.UpdateSetting)
				r.Get("/admin/users/{id}/settings", adminHandler.GetUserSettings)
				r.Patch("/admin/users/{id}/settings", adminHandler.UpdateUserSettings)
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
