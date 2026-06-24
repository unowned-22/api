package bootstrap

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/auth"
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/domain/album"
	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/domain/photo"
	"github.com/unowned-22/api/internal/domain/photocomment"
	domainstorage "github.com/unowned-22/api/internal/domain/storage"
	"github.com/unowned-22/api/internal/domain/systemsettings"
	"github.com/unowned-22/api/internal/domain/token"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/domain/usersettings"
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
	UserSettings   usersettings.Service
	Story          *service.StoryService
	Friendship     *service.FriendshipService
	Profile        *service.ProfileService
	Notification   notification.Service
	Photo          photo.Service
	Album          album.Service
	PhotoComment   photocomment.Service
}

// InitServices constructs application services from repositories and infra.
func InitServices(
	cfg *config.Config,
	pool *pgxpool.Pool,
	repos *Repositories,
	tokenManager token.ManagerExtended,
	smtp *mailer.SMTPMailer,
	publisher *queue.AMQPPublisher,
	storage domainstorage.Storage,
	tokenVersionCache user.TokenVersionCache,
) *Services {
	// create an outbox-backed publisher that persists events into the outbox table
	outboxPublisher := outboxpub.New(repos.Outbox)
	authSvc := auth.NewAuthService(
		repos.User,
		repos.RefreshToken,
		repos.UserSession,
		repos.UserDevice,
		repos.Role,
		tokenManager,
		smtp,
		outboxPublisher,
		cfg.RefreshTokenTTL,
		cfg.AppURL,
		cfg.AppName,
		tokenVersionCache,
		pool,
	)

	passwordResetSvc := service.NewPasswordResetService(repos.User, repos.PasswordReset, repos.RefreshToken, repos.UserSession, smtp, outboxPublisher, cfg.AppURL, cfg.AppName)
	userSvc := service.NewUserService(repos.User, storage, repos.UserSettings, cfg.MinIOBucket)
	permissionSvc := service.NewPermissionService(repos.Permission)
	healthSvc := service.NewHealthService(pool)
	systemSettingsSvc := service.NewSystemSettingsService(repos.SystemSettings)
	userSettingsSvc := service.NewUserSettingsService(repos.UserSettings)
	friendshipSvc := service.NewFriendshipService(repos.Friendship, outboxPublisher)
	storySvc := service.NewStoryService(repos.Story, friendshipSvc, outboxPublisher)
	notifSvc := service.NewNotificationService(repos.Notification)
	profileSvc := service.NewProfileService(repos.User, repos.Friendship, repos.UserPrivacy, friendshipSvc)
	photoSvc := service.NewPhotoService(repos.Photo, repos.Album, repos.UserSettings, storage, notifSvc)
	// initialize photo comment service
	photoCommentSvc := service.NewPhotoCommentService(repos.PhotoComment, repos.Photo, repos.Notification)
	albumSvc := service.NewAlbumService(repos.Album, repos.Photo)

	return &Services{
		Auth:           authSvc,
		PasswordReset:  passwordResetSvc,
		User:           userSvc,
		Permission:     permissionSvc,
		Health:         healthSvc,
		SystemSettings: systemSettingsSvc,
		UserSettings:   userSettingsSvc,
		Story:          storySvc,
		Friendship:     friendshipSvc,
		Profile:        profileSvc,
		Notification:   notifSvc,
		Photo:          photoSvc,
		PhotoComment:   photoCommentSvc,
		Album:          albumSvc,
	}
}
