package bootstrap

import (
	"github.com/jackc/pgx/v5/pgxpool"
	domout "github.com/unowned-22/api/internal/domain/outbox"
	postgresRepo "github.com/unowned-22/api/internal/repository/postgres"
	outboxRepo "github.com/unowned-22/api/internal/repository/postgres/outbox"
)

type Repositories struct {
	User           *postgresRepo.UserRepository
	RefreshToken   *postgresRepo.RefreshTokenRepository
	UserSession    *postgresRepo.UserSessionRepository
	Role           *postgresRepo.RoleRepository
	Permission     *postgresRepo.PermissionRepository
	PasswordReset  *postgresRepo.PasswordResetRepository
	Audit          *postgresRepo.AuditRepository
	SystemSettings *postgresRepo.SystemSettingsRepository
	UserSettings   *postgresRepo.UserSettingsRepository
	UserDevice     *postgresRepo.UserDeviceRepository
	Notification   *postgresRepo.NotificationRepository
	Outbox         domout.Repository
	Story          *postgresRepo.StoryRepository
	Friendship     *postgresRepo.FriendshipRepository
	CloseFriend    *postgresRepo.CloseFriendRepository
	UserPrivacy    *postgresRepo.UserPrivacyRepository
	Photo          *postgresRepo.PhotoRepository
	PhotoComment   *postgresRepo.PhotoCommentRepository
	Album          *postgresRepo.AlbumRepository
}

// InitRepositories wires repository implementations using the provided pool.
func InitRepositories(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		User:           postgresRepo.NewUserRepository(pool),
		RefreshToken:   postgresRepo.NewRefreshTokenRepository(pool),
		UserSession:    postgresRepo.NewUserSessionRepository(pool),
		Role:           postgresRepo.NewRoleRepository(pool),
		Permission:     postgresRepo.NewPermissionRepository(pool),
		PasswordReset:  postgresRepo.NewPasswordResetRepository(pool),
		Audit:          postgresRepo.NewAuditRepository(pool),
		SystemSettings: postgresRepo.NewSystemSettingsRepository(pool),
		UserSettings:   postgresRepo.NewUserSettingsRepository(pool),
		UserDevice:     postgresRepo.NewUserDeviceRepository(pool),
		Notification:   postgresRepo.NewNotificationRepository(pool),
		Outbox:         outboxRepo.NewRepository(pool),
		Story:          postgresRepo.NewStoryRepository(pool),
		Friendship:     postgresRepo.NewFriendshipRepository(pool),
		CloseFriend:    postgresRepo.NewCloseFriendRepository(pool),
		UserPrivacy:    postgresRepo.NewUserPrivacyRepository(pool),
		Photo:          postgresRepo.NewPhotoRepository(pool),
		PhotoComment:   postgresRepo.NewPhotoCommentRepository(pool),
		Album:          postgresRepo.NewAlbumRepository(pool),
	}
}
