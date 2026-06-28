package bootstrap

import (
	"github.com/jackc/pgx/v5/pgxpool"
	domout "github.com/unowned-22/api/internal/domain/outbox"
	postgresRepo "github.com/unowned-22/api/internal/repository/postgres"
	messengerRepo "github.com/unowned-22/api/internal/repository/postgres/messenger"
	outboxRepo "github.com/unowned-22/api/internal/repository/postgres/outbox"
)

type Repositories struct {
	User              *postgresRepo.UserRepository
	RefreshToken      *postgresRepo.RefreshTokenRepository
	UserSession       *postgresRepo.UserSessionRepository
	PasswordReset     *postgresRepo.PasswordResetRepository
	Audit             *postgresRepo.AuditRepository
	SystemSettings    *postgresRepo.SystemSettingsRepository
	UserSettings      *postgresRepo.UserSettingsRepository
	UserDevice        *postgresRepo.UserDeviceRepository
	Notification      *postgresRepo.NotificationRepository
	Outbox            domout.Repository
	Story             *postgresRepo.StoryRepository
	Friendship        *postgresRepo.FriendshipRepository
	CloseFriend       *postgresRepo.CloseFriendRepository
	UserPrivacy       *postgresRepo.UserPrivacyRepository
	Photo             *postgresRepo.PhotoRepository
	PhotoComment      *postgresRepo.PhotoCommentRepository
	VideoChannel      *postgresRepo.VideoChannelRepository
	Video             *postgresRepo.VideoRepository
	VideoComment      *postgresRepo.VideoCommentRepository
	VideoPlaylist     *postgresRepo.VideoPlaylistRepository
	VideoSubscription *postgresRepo.VideoSubscriptionRepository
	Album             *postgresRepo.AlbumRepository
	Conversation      *messengerRepo.ConversationRepository
	Message           *messengerRepo.MessageRepository
	Member            *messengerRepo.MemberRepository
	Presence          *messengerRepo.PresenceRepository
	MessengerPrivacy  *messengerRepo.PrivacyRepository
	Draft             *messengerRepo.DraftRepository
}

// InitRepositories wires repository implementations using the provided pool.
func InitRepositories(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		User:              postgresRepo.NewUserRepository(pool),
		RefreshToken:      postgresRepo.NewRefreshTokenRepository(pool),
		UserSession:       postgresRepo.NewUserSessionRepository(pool),
		PasswordReset:     postgresRepo.NewPasswordResetRepository(pool),
		Audit:             postgresRepo.NewAuditRepository(pool),
		SystemSettings:    postgresRepo.NewSystemSettingsRepository(pool),
		UserSettings:      postgresRepo.NewUserSettingsRepository(pool),
		UserDevice:        postgresRepo.NewUserDeviceRepository(pool),
		Notification:      postgresRepo.NewNotificationRepository(pool),
		Outbox:            outboxRepo.NewRepository(pool),
		Story:             postgresRepo.NewStoryRepository(pool),
		Friendship:        postgresRepo.NewFriendshipRepository(pool),
		CloseFriend:       postgresRepo.NewCloseFriendRepository(pool),
		UserPrivacy:       postgresRepo.NewUserPrivacyRepository(pool),
		Photo:             postgresRepo.NewPhotoRepository(pool),
		PhotoComment:      postgresRepo.NewPhotoCommentRepository(pool),
		VideoChannel:      postgresRepo.NewVideoChannelRepository(pool),
		Video:             postgresRepo.NewVideoRepository(pool),
		VideoComment:      postgresRepo.NewVideoCommentRepository(pool),
		VideoPlaylist:     postgresRepo.NewVideoPlaylistRepository(pool),
		VideoSubscription: postgresRepo.NewVideoSubscriptionRepository(pool),
		Album:             postgresRepo.NewAlbumRepository(pool),
		Conversation:      messengerRepo.NewConversationRepository(pool),
		Message:           messengerRepo.NewMessageRepository(pool),
		Member:            messengerRepo.NewMemberRepository(pool),
		Presence:          messengerRepo.NewPresenceRepository(pool),
		MessengerPrivacy:  messengerRepo.NewPrivacyRepository(pool),
		Draft:             messengerRepo.NewDraftRepository(pool),
	}
}
