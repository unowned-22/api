package bootstrap

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/auth"
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/domain/album"
	"github.com/unowned-22/api/internal/domain/closefriend"
	"github.com/unowned-22/api/internal/domain/community"
	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/media"
	"github.com/unowned-22/api/internal/domain/messenger"
	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/domain/photo"
	"github.com/unowned-22/api/internal/domain/photocomment"
	domainsearch "github.com/unowned-22/api/internal/domain/search"
	domainstorage "github.com/unowned-22/api/internal/domain/storage"
	"github.com/unowned-22/api/internal/domain/systemsettings"
	"github.com/unowned-22/api/internal/domain/token"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/domain/usersettings"
	"github.com/unowned-22/api/internal/domain/video"
	"github.com/unowned-22/api/internal/domain/videocomment"
	"github.com/unowned-22/api/internal/domain/videoplaylist"
	"github.com/unowned-22/api/internal/infrastructure/mailer"
	outboxpub "github.com/unowned-22/api/internal/infrastructure/outbox"
	"github.com/unowned-22/api/internal/infrastructure/queue"
	"github.com/unowned-22/api/internal/service"
)

type Services struct {
	Auth            auth.AuthService
	PasswordReset   service.PasswordResetService
	User            *service.UserService
	Health          *service.HealthService
	SystemSettings  systemsettings.Service
	UserSettings    usersettings.Service
	Story           *service.StoryService
	Friendship      *service.FriendshipService
	CloseFriend     closefriend.Service
	Profile         *service.ProfileService
	Notification    notification.Service
	Photo           photo.Service
	Album           album.Service
	PhotoComment    photocomment.Service
	Community       community.Service
	Video           video.Service
	VideoComment    videocomment.Service
	VideoPlaylist   videoplaylist.Service
	Messenger       messenger.Service
	OutboxPublisher event.Publisher
	UserSearchIndex domainsearch.UserIndex
	UserSearch      *service.UserSearchService
	Post            *service.PostService
	Feed            *service.FeedService
}

func InitServices(
	cfg *config.Config,
	pool *pgxpool.Pool,
	repos *Repositories,
	tokenManager token.ManagerExtended,
	smtp *mailer.SMTPMailer,
	publisher *queue.AMQPPublisher,
	storage domainstorage.Storage,
	tokenVersionCache user.TokenVersionCache,
	userSearchIndex domainsearch.UserIndex,
	imageProcessor *media.Processor,
) *Services {
	outboxPublisher := outboxpub.New(repos.Outbox)
	authSvc := auth.NewAuthService(
		repos.User,
		repos.RefreshToken,
		repos.UserSession,
		repos.UserDevice,
		tokenManager,
		smtp,
		outboxPublisher,
		cfg.RefreshTokenTTL,
		cfg.AppURL,
		cfg.AppName,
		tokenVersionCache,
		pool,
	)

	jobQueue := queue.NewVideoJobQueue(publisher, "video.processing_progress")

	passwordResetSvc := service.NewPasswordResetService(repos.User, repos.PasswordReset, repos.RefreshToken, repos.UserSession, smtp, outboxPublisher, cfg.AppURL, cfg.AppName)
	userSvc := service.NewUserService(repos.User, storage, repos.UserSettings, cfg.MinIOBucket, imageProcessor, outboxPublisher)
	userSearchSvc := service.NewUserSearchService(userSearchIndex)
	healthSvc := service.NewHealthService(pool)
	systemSettingsSvc := service.NewSystemSettingsService(repos.SystemSettings)
	userSettingsSvc := service.NewUserSettingsService(repos.UserSettings)
	friendshipSvc := service.NewFriendshipService(repos.Friendship, outboxPublisher)
	closeFriendSvc := service.NewCloseFriendService(repos.CloseFriend, repos.Friendship)
	notifSvc := service.NewNotificationService(repos.Notification)
	profileSvc := service.NewProfileService(repos.User, repos.Friendship, repos.UserPrivacy, friendshipSvc)
	photoSvc := service.NewPhotoService(repos.Photo, repos.Album, repos.UserSettings, storage, outboxPublisher, cfg.MinIOBucket)
	photoCommentSvc := service.NewPhotoCommentService(repos.PhotoComment, repos.Photo, outboxPublisher)
	communitySvc := service.NewCommunityService(repos.Community, repos.Conversation)
	videoSvc := service.NewVideoService(repos.Video, communitySvc, repos.CommunityPost, storage, jobQueue, cfg.MinIOBucket, outboxPublisher, 10737418240)
	videoCommentSvc := service.NewVideoCommentService(repos.VideoComment, repos.Video, outboxPublisher)
	videoPlaylistSvc := service.NewVideoPlaylistService(repos.VideoPlaylist, repos.Video)
	albumSvc := service.NewAlbumService(repos.Album, repos.Photo)
	messengerSvc := service.NewMessengerService(
		repos.Conversation, repos.Message, repos.Member, repos.Presence,
		repos.MessengerPrivacy, repos.Draft, friendshipSvc, storage, cfg.MinIOBucket,
		outboxPublisher,
		communitySvc,
	)

	postSvc := service.NewPostService(repos.UserPost, repos.CommunityPost, communitySvc, outboxPublisher)
	feedSvc := service.NewFeedService(repos.Feed)
	storySvc := service.NewStoryService(repos.Story, friendshipSvc, outboxPublisher, communitySvc)

	return &Services{
		Auth:            authSvc,
		PasswordReset:   passwordResetSvc,
		User:            userSvc,
		Health:          healthSvc,
		SystemSettings:  systemSettingsSvc,
		UserSettings:    userSettingsSvc,
		Story:           storySvc,
		Friendship:      friendshipSvc,
		CloseFriend:     closeFriendSvc,
		Profile:         profileSvc,
		Notification:    notifSvc,
		Photo:           photoSvc,
		PhotoComment:    photoCommentSvc,
		Community:       communitySvc,
		Video:           videoSvc,
		VideoComment:    videoCommentSvc,
		VideoPlaylist:   videoPlaylistSvc,
		Album:           albumSvc,
		Messenger:       messengerSvc,
		OutboxPublisher: outboxPublisher,
		UserSearchIndex: userSearchIndex,
		UserSearch:      userSearchSvc,
		Post:            postSvc,
		Feed:            feedSvc,
	}
}
