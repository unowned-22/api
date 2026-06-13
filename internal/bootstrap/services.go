package bootstrap

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/auth"
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/domain/systemsettings"
	"github.com/unowned-22/api/internal/domain/token"
	"github.com/unowned-22/api/internal/infrastructure/mailer"
	outboxpub "github.com/unowned-22/api/internal/infrastructure/outbox"
	"github.com/unowned-22/api/internal/infrastructure/queue"
	"github.com/unowned-22/api/internal/service"
)

type Services struct {
	Auth           auth.AuthService
	PasswordReset  service.PasswordResetService
	User           *service.UserService
	Permission     *service.PermissionService
	Health         *service.HealthService
	SystemSettings systemsettings.Service
}

// InitServices constructs application services from repositories and infra.
func InitServices(cfg *config.Config, pool *pgxpool.Pool, repos *Repositories, tokenManager token.ManagerExtended, smtp *mailer.SMTPMailer, publisher *queue.AMQPPublisher) *Services {
	// create an outbox-backed publisher that persists events into the outbox table
	outboxPublisher := outboxpub.New(repos.Outbox)
	authSvc := auth.NewAuthService(
		repos.User,
		repos.RefreshToken,
		repos.UserSession,
		repos.Role,
		tokenManager,
		smtp,
		outboxPublisher,
		cfg.RefreshTokenTTL,
		cfg.AppURL,
		cfg.AppName,
	)

	passwordResetSvc := service.NewPasswordResetService(repos.User, repos.PasswordReset, repos.RefreshToken, repos.UserSession, smtp, outboxPublisher, cfg.AppURL, cfg.AppName)
	userSvc := service.NewUserService(repos.User)
	permissionSvc := service.NewPermissionService(repos.Permission)
	healthSvc := service.NewHealthService(pool)
	systemSettingsSvc := service.NewSystemSettingsService(repos.SystemSettings)

	return &Services{
		Auth:           authSvc,
		PasswordReset:  passwordResetSvc,
		User:           userSvc,
		Permission:     permissionSvc,
		Health:         healthSvc,
		SystemSettings: systemSettingsSvc,
	}
}
