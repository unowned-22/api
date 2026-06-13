package bootstrap

import (
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/infrastructure/storage"
	"github.com/unowned-22/api/internal/transport/http/handler"
)

type Handlers struct {
	Auth          *handler.AuthHandler
	PasswordReset *handler.PasswordResetHandler
	User          *handler.UserHandler
	Admin         *handler.AdminHandler
	Health        *handler.HealthHandler
	Upload        *handler.UploadHandler
}

// InitHandlers wires HTTP handlers from services and infra.
func InitHandlers(cfg *config.Config, svcs *Services, storage *storage.MinIOStorage) *Handlers {
	authHandler := handler.NewAuthHandler(svcs.Auth)
	passwordResetHandler := handler.NewPasswordResetHandler(svcs.PasswordReset)
	userHandler := handler.NewUserHandler(svcs.User)
	adminHandler := handler.NewAdminHandler(svcs.User, svcs.Permission, svcs.Auth, svcs.SystemSettings)
	healthHandler := handler.NewHealthHandler(svcs.Health)
	uploadHandler := handler.NewUploadHandler(storage, cfg.MinIOBucket)

	return &Handlers{
		Auth:          authHandler,
		PasswordReset: passwordResetHandler,
		User:          userHandler,
		Admin:         adminHandler,
		Health:        healthHandler,
		Upload:        uploadHandler,
	}
}
