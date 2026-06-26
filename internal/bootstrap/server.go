package bootstrap

import (
	"net/http"
	"time"

	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/domain/token"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/middleware"
	transportHttp "github.com/unowned-22/api/internal/transport/http"
)

// NewServer builds the HTTP server and its auth rate limiters.
func NewServer(
	cfg *config.Config,
	h *Handlers,
	tokenManager token.Manager,
	svcs *Services,
	tokenVersionCache user.TokenVersionCache,
) (*http.Server, *middleware.AuthRateLimiter, *middleware.AuthRateLimiter, *middleware.AuthRateLimiter, *middleware.AuthRateLimiter) {
	loginLimiter := middleware.NewAuthRateLimiter(middleware.AuthRateLimiterConfig{Limit: cfg.LoginRateLimit, Window: cfg.LoginRateLimitWindow})
	registerLimiter := middleware.NewAuthRateLimiter(middleware.AuthRateLimiterConfig{Limit: cfg.RegisterRateLimit, Window: cfg.RegisterRateLimitWindow})
	forgotLimiter := middleware.NewAuthRateLimiter(middleware.AuthRateLimiterConfig{Limit: cfg.ForgotPasswordRateLimit, Window: cfg.ForgotPasswordRateLimitWindow})
	resendLimiter := middleware.NewAuthRateLimiter(middleware.AuthRateLimiterConfig{Limit: cfg.ResendVerificationRateLimit, Window: cfg.ResendVerificationRateLimitWindow})

	router := transportHttp.NewRouter(cfg, h.Auth, h.User, h.PasswordReset, h.Admin, h.Upload, h.Health, h.Story, h.Friendship, h.Profile, h.Notification, h.Photo, h.Album, h.PhotoComment, h.CloseFriend, h.Messenger, tokenManager, svcs.User, svcs.Permission, loginLimiter, registerLimiter, forgotLimiter, resendLimiter, tokenVersionCache)

	srv := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           router,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return srv, loginLimiter, registerLimiter, forgotLimiter, resendLimiter
}
