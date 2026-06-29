package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/config"
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
	uploadHandler *handler.UploadHandler,
	healthHandler *handler.HealthHandler,
	storyHandler *handler.StoryHandler,
	friendshipHandler *handler.FriendshipHandler,
	profileHandler *handler.ProfileHandler,
	notificationHandler *handler.NotificationHandler,
	photoHandler *handler.PhotoHandler,
	albumHandler *handler.AlbumHandler,
	photoCommentHandler *handler.PhotoCommentHandler,
	videoChannelHandler *handler.VideoChannelHandler,
	videoHandler *handler.VideoHandler,
	videoCommentHandler *handler.VideoCommentHandler,
	videoPlaylistHandler *handler.VideoPlaylistHandler,
	videoSubscriptionHandler *handler.VideoSubscriptionHandler,
	closeFriendHandler *handler.CloseFriendHandler,
	messengerHandler *handler.MessengerHandler,
	userSearchHandler *handler.UserSearchHandler,
	tokenManager token.Manager,
	userService user.UserService,
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
			r.Get("/users/search", userSearchHandler.Search)
			r.Patch("/users/me", userHandler.UpdateProfile)
			r.Put("/users/me/password", authHandler.ChangePassword)
			r.Get("/users/me/settings", userHandler.GetMySettings)
			r.Patch("/users/me/settings", userHandler.UpdateMySettings)
			r.Patch("/users/me/settings/notifications", userHandler.UpdateMyNotificationPreferences)
			r.Get("/users/me/close-friends", closeFriendHandler.List)
			r.Post("/users/me/close-friends", closeFriendHandler.Add)
			r.Delete("/users/me/close-friends/{friendID}", closeFriendHandler.Remove)

			r.Get("/messenger/privacy", messengerHandler.GetPrivacy)
			r.Put("/messenger/privacy", messengerHandler.UpdatePrivacy)
			r.Post("/messenger/blocked/{userID}", messengerHandler.BlockUser)
			r.Delete("/messenger/blocked/{userID}", messengerHandler.UnblockUser)
			r.Get("/messenger/blocked", messengerHandler.ListBlocked)
			r.Post("/messenger/conversations/direct/{userID}", messengerHandler.GetOrCreateDirect)
			r.Post("/messenger/conversations/group", messengerHandler.CreateGroup)
			r.Post("/messenger/conversations/channel", messengerHandler.CreateChannel)
			r.Get("/messenger/conversations", messengerHandler.ListConversations)
			r.Get("/messenger/conversations/{id}", messengerHandler.GetConversation)
			r.Post("/messenger/conversations/{id}/members", messengerHandler.AddMembers)
			r.Delete("/messenger/conversations/{id}/members/{userID}", messengerHandler.RemoveMember)
			r.Post("/messenger/conversations/{id}/leave", messengerHandler.LeaveConversation)
			r.Post("/messenger/conversations/{id}/subscribe", messengerHandler.Subscribe)
			r.Post("/messenger/conversations/{id}/archive", messengerHandler.ArchiveConversation)
			r.Post("/messenger/conversations/{id}/unarchive", messengerHandler.UnarchiveConversation)
			r.Post("/messenger/conversations/{id}/invite", messengerHandler.GenerateInviteLink)
			r.Delete("/messenger/conversations/{id}/invite", messengerHandler.RevokeInviteLink)
			r.Post("/messenger/join/{slug}", messengerHandler.JoinByInviteLink)
			r.Post("/messenger/conversations/{id}/messages", messengerHandler.SendMessage)
			r.Get("/messenger/conversations/{id}/messages", messengerHandler.ListMessages)
			r.Get("/messenger/conversations/{id}/messages/search", messengerHandler.SearchMessages)
			r.Get("/messenger/conversations/{id}/messages/pinned", messengerHandler.ListPinned)
			r.Patch("/messenger/messages/{id}", messengerHandler.EditMessage)
			r.Delete("/messenger/messages/{id}", messengerHandler.DeleteMessage)
			r.Post("/messenger/messages/{id}/pin", messengerHandler.PinMessage)
			r.Delete("/messenger/messages/{id}/pin", messengerHandler.UnpinMessage)
			r.Post("/messenger/messages/{id}/forward", messengerHandler.ForwardMessage)
			r.Post("/messenger/conversations/{id}/read", messengerHandler.MarkRead)
			r.Post("/messenger/conversations/{id}/messages/schedule", messengerHandler.ScheduleMessage)
			r.Get("/messenger/conversations/{id}/messages/scheduled", messengerHandler.ListScheduled)
			r.Delete("/messenger/scheduled/{id}", messengerHandler.CancelScheduled)
			r.Put("/messenger/conversations/{id}/draft", messengerHandler.SaveDraft)
			r.Get("/messenger/conversations/{id}/draft", messengerHandler.GetDraft)
			r.Delete("/messenger/conversations/{id}/draft", messengerHandler.DeleteDraft)
			r.Put("/messenger/conversations/{id}/disappear-timer", messengerHandler.SetDisappearTimer)
			r.Get("/messenger/mentions", messengerHandler.ListMentions)
			r.Post("/messenger/attachments/upload", messengerHandler.UploadAttachment)
			r.Post("/messenger/conversations/{id}/typing", messengerHandler.Typing)
			r.Post("/messenger/messages/{id}/reactions", messengerHandler.AddReaction)
			r.Delete("/messenger/messages/{id}/reactions/{emoji}", messengerHandler.RemoveReaction)

			r.Get("/auth/sessions", authHandler.ListSessions)
			r.Delete("/auth/sessions/{id}", authHandler.RevokeSession)
			r.Post("/auth/logout-all", authHandler.LogoutAll)
			r.Post("/users/me/avatar", uploadHandler.UploadAvatar)
			r.Post("/users/me/cover", uploadHandler.UploadCover)
			r.Delete("/users/me/avatar", uploadHandler.DeleteAvatar)
			r.Delete("/users/me/cover", uploadHandler.DeleteCover)
			r.With(middleware.RateLimit(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst)).Post("/stories", storyHandler.Publish)
			r.Get("/stories/me", storyHandler.ListMine)
			r.Get("/stories/feed", storyHandler.Feed)
			r.Delete("/stories/{id}", storyHandler.Delete)
			r.With(middleware.RateLimit(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst)).Post("/stories/{id}/view", storyHandler.View)
			r.With(middleware.RateLimit(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst)).Post("/stories/{id}/like", storyHandler.Like)
			r.With(middleware.RateLimit(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst)).Post("/stories/{id}/unlike", storyHandler.Unlike)
			r.With(middleware.RateLimit(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst)).Post("/stories/{id}/reply", storyHandler.Reply)
			r.With(middleware.RateLimit(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst)).Post("/stories/media", uploadHandler.UploadStoryMedia)

			r.Post("/photos", photoHandler.UploadPhoto)
			r.Get("/photos", photoHandler.ListMyPhotos)
			r.Get("/photos/{photoID}", photoHandler.GetPhoto)
			r.Patch("/photos/{photoID}", photoHandler.UpdatePhotoMeta)
			r.Patch("/photos/{photoID}/move", photoHandler.MovePhotoToAlbum)
			r.Delete("/photos/{photoID}", photoHandler.DeletePhoto)
			r.Post("/photos/{photoID}/like", photoCommentHandler.LikePhoto)
			r.Delete("/photos/{photoID}/like", photoCommentHandler.UnlikePhoto)

			r.Post("/channels", videoChannelHandler.CreateMyChannel)
			r.Get("/channels/me", videoChannelHandler.GetMyChannel)
			r.Patch("/channels/me", videoChannelHandler.UpdateMyChannel)
			r.Post("/channels/me/avatar", videoChannelHandler.UploadAvatar)
			r.Post("/channels/me/banner", videoChannelHandler.UploadBanner)
			r.Get("/channels/{id}", videoChannelHandler.GetChannel)
			r.Get("/channels/{id}/videos", videoChannelHandler.ListChannelVideos)
			r.Post("/channels/{id}/subscribe", videoSubscriptionHandler.Subscribe)
			r.Delete("/channels/{id}/subscribe", videoSubscriptionHandler.Unsubscribe)
			r.Get("/channels/{id}/subscribers", videoSubscriptionHandler.GetSubscriberCount)
			r.Get("/users/me/subscriptions", videoSubscriptionHandler.ListMySubscriptions)
			r.Post("/videos", videoHandler.UploadVideo)
			r.Get("/videos/feed", videoHandler.VideoFeed)
			r.Get("/videos/search", videoHandler.SearchVideos)
			r.Get("/videos/{id}", videoHandler.GetVideo)
			r.Patch("/videos/{id}", videoHandler.UpdateVideo)
			r.Delete("/videos/{id}", videoHandler.DeleteVideo)
			r.Post("/videos/{id}/view", videoHandler.RecordView)
			r.Post("/videos/{id}/like", videoHandler.LikeVideo)
			r.Delete("/videos/{id}/like", videoHandler.UnlikeVideo)
			r.Get("/videos/{videoID}/comments", videoCommentHandler.ListComments)
			r.Post("/videos/{videoID}/comments", videoCommentHandler.AddComment)
			r.Delete("/videos/{videoID}/comments/{commentID}", videoCommentHandler.DeleteComment)
			r.Get("/videos/{videoID}/comments/{commentID}/replies", videoCommentHandler.ListReplies)
			r.Post("/videos/{videoID}/comments/{commentID}/like", videoCommentHandler.LikeComment)
			r.Delete("/videos/{videoID}/comments/{commentID}/like", videoCommentHandler.UnlikeComment)
			r.Get("/playlists", videoPlaylistHandler.ListMyPlaylists)
			r.Post("/playlists", videoPlaylistHandler.CreatePlaylist)
			r.Get("/playlists/{id}", videoPlaylistHandler.GetPlaylist)
			r.Patch("/playlists/{id}", videoPlaylistHandler.UpdatePlaylist)
			r.Delete("/playlists/{id}", videoPlaylistHandler.DeletePlaylist)
			r.Post("/playlists/{id}/items", videoPlaylistHandler.AddVideoToPlaylist)
			r.Delete("/playlists/{id}/items/{videoID}", videoPlaylistHandler.RemoveVideoFromPlaylist)
			r.Get("/playlists/{id}/items", videoPlaylistHandler.ListPlaylistItems)

			r.Post("/albums", albumHandler.CreateAlbum)
			r.Get("/albums", albumHandler.ListMyAlbums)
			r.Get("/albums/{albumID}", albumHandler.GetAlbum)
			r.Patch("/albums/{albumID}", albumHandler.UpdateAlbum)
			r.Delete("/albums/{albumID}", albumHandler.DeleteAlbum)
			r.Patch("/albums/{albumID}/cover", albumHandler.SetAlbumCover)
			r.Get("/albums/{albumID}/photos", albumHandler.ListAlbumPhotos)

			r.Post("/photos/{photoID}/comments", photoCommentHandler.AddComment)
			r.Get("/photos/{photoID}/comments", photoCommentHandler.ListComments)
			r.Get("/photos/comments/{commentID}/replies", photoCommentHandler.ListReplies)
			r.Patch("/photos/comments/{commentID}", photoCommentHandler.EditComment)
			r.Delete("/photos/comments/{commentID}", photoCommentHandler.DeleteComment)
			r.Post("/photos/comments/{commentID}/like", photoCommentHandler.LikeComment)
			r.Delete("/photos/comments/{commentID}/like", photoCommentHandler.UnlikeComment)

			r.Get("/notifications", notificationHandler.List)
			r.Get("/notifications/unread-count", notificationHandler.UnreadCount)
			r.Post("/notifications/{id}/read", notificationHandler.MarkRead)
			r.Post("/notifications/read-all", notificationHandler.MarkAllRead)

			r.Post("/friends/requests", friendshipHandler.SendRequest)
			r.Post("/friends/requests/{id}/accept", friendshipHandler.Accept)
			r.Post("/friends/requests/{id}/reject", friendshipHandler.Reject)
			r.Post("/friends/requests/{id}/cancel", friendshipHandler.Cancel)
			r.Get("/friends/requests/incoming", friendshipHandler.ListIncoming)
			r.Get("/friends/requests/outgoing", friendshipHandler.ListOutgoing)
			r.Get("/friends/suggestions", friendshipHandler.ListSuggestions)
			r.Get("/friends", friendshipHandler.ListFriends)
			r.Delete("/friends/{id}", friendshipHandler.Remove)

			r.Get("/users/{username}", profileHandler.GetByUsername)
			r.Get("/users/{username}/photos", photoHandler.ListUserPhotosByUsername)
			r.Get("/users/{username}/albums", albumHandler.ListUserAlbumsByUsername)
		})
	})

	return r
}
