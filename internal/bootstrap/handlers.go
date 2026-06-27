package bootstrap

import (
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/infrastructure/storage"
	"github.com/unowned-22/api/internal/transport/http/handler"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

type Handlers struct {
	Auth          *handler.AuthHandler
	PasswordReset *handler.PasswordResetHandler
	User          *handler.UserHandler
	Health        *handler.HealthHandler
	Upload        *handler.UploadHandler
	Story         *handler.StoryHandler
	Friendship    *handler.FriendshipHandler
	Profile       *handler.ProfileHandler
	Notification  *handler.NotificationHandler
	Photo         *handler.PhotoHandler
	Album         *handler.AlbumHandler
	PhotoComment  *handler.PhotoCommentHandler
	CloseFriend   *handler.CloseFriendHandler
	Messenger     *handler.MessengerHandler
}

// InitHandlers wires HTTP handlers from services and infra.
func InitHandlers(cfg *config.Config, svcs *Services, storage *storage.MinIOStorage, hub *ws.Hub) *Handlers {
	authHandler := handler.NewAuthHandler(svcs.Auth)
	passwordResetHandler := handler.NewPasswordResetHandler(svcs.PasswordReset)
	userHandler := handler.NewUserHandler(svcs.User, svcs.UserSettings)
	healthHandler := handler.NewHealthHandler(svcs.Health)
	uploadHandler := handler.NewUploadHandler(storage, cfg.MinIOBucket, svcs.User)
	storyHandler := handler.NewStoryHandler(svcs.Story, storage, cfg.MinIOBucket, svcs.User)
	friendshipHandler := handler.NewFriendshipHandler(svcs.Friendship)
	profileHandler := handler.NewProfileHandler(svcs.Profile)
	notificationHandler := handler.NewNotificationHandler(svcs.Notification, hub)
	photoHandler := handler.NewPhotoHandler(svcs.Photo, svcs.Album, svcs.Profile)
	albumHandler := handler.NewAlbumHandler(svcs.Album, svcs.Photo, svcs.Profile)
	photoCommentHandler := handler.NewPhotoCommentHandler(svcs.PhotoComment, svcs.Photo)
	closeFriendHandler := handler.NewCloseFriendHandler(svcs.CloseFriend)
	messengerHandler := handler.NewMessengerHandler(svcs.Messenger, storage, *cfg)

	return &Handlers{
		Auth:          authHandler,
		PasswordReset: passwordResetHandler,
		User:          userHandler,
		Health:        healthHandler,
		Upload:        uploadHandler,
		Story:         storyHandler,
		Friendship:    friendshipHandler,
		Profile:       profileHandler,
		Notification:  notificationHandler,
		Photo:         photoHandler,
		PhotoComment:  photoCommentHandler,
		Album:         albumHandler,
		CloseFriend:   closeFriendHandler,
		Messenger:     messengerHandler,
	}
}
