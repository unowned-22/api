package errs

import "errors"

var (
	ErrUserNotFound              = errors.New("user not found")
	ErrInvalidCredentials        = errors.New("invalid credentials")
	ErrUserAlreadyExists         = errors.New("user already exists")
	ErrUsernameAlreadyExists     = errors.New("username already exists")
	ErrInvalidRefreshToken       = errors.New("refresh token is invalid")
	ErrRefreshTokenNotFound      = errors.New("refresh token not found")
	ErrRoleNotFound              = errors.New("role not found")
	ErrForbidden                 = errors.New("forbidden")
	ErrVerificationTokenInvalid  = errors.New("verification token is invalid or expired")
	ErrEmailAlreadyVerified      = errors.New("email already verified")
	ErrPasswordResetTokenInvalid = errors.New("password reset token is invalid or expired")
	ErrPasswordResetTokenUsed    = errors.New("password reset token has already been used")
	ErrEmailNotVerified          = errors.New("email not verified")
	ErrSessionNotFound           = errors.New("session not found")
	ErrUserDeactivated           = errors.New("user account is deactivated")

	// ErrUserStorageNotProvisioned is returned when the user's MinIO bucket or
	// user_settings row does not yet exist. This is a transient condition: the
	// email_verified worker creates the bucket asynchronously after verification,
	// so the provisioning may still be in flight (or may have failed and ended up
	// in the DLQ). The transport layer maps this to 503 Service Unavailable so
	// clients know to retry rather than treat the situation as a permanent error.
	ErrUserStorageNotProvisioned = errors.New("user storage is not yet provisioned")

	// ErrUserSettingsNotFound is returned when a user_settings row is expected
	// to exist but cannot be found. Semantically distinct from
	// ErrUserStorageNotProvisioned: use this when the lookup itself indicates a
	// missing record rather than an incomplete provisioning flow.
	ErrUserSettingsNotFound = errors.New("user settings not found")

	// ErrAvatarNotFound is returned when a user attempts to delete an avatar
	// that has not been uploaded.
	ErrAvatarNotFound = errors.New("avatar not found")

	// ErrCoverNotFound is returned when a user attempts to delete a cover
	// that has not been uploaded.
	ErrCoverNotFound = errors.New("cover not found")

	// Stories
	ErrStoryNotFound       = errors.New("story not found")
	ErrInvalidStoryPayload = errors.New("invalid story payload")

	// Messenger
	ErrConversationNotFound    = errors.New("conversation not found")
	ErrMessageNotFound         = errors.New("message not found")
	ErrMemberNotFound          = errors.New("conversation member not found")
	ErrPresenceNotFound        = errors.New("user presence not found")
	ErrPrivacySettingsNotFound = errors.New("messenger privacy settings not found")
	ErrDraftNotFound           = errors.New("message draft not found")
	ErrCannotBlockSelf         = errors.New("cannot block yourself")
	ErrMessagingNotAllowed     = errors.New("messaging is not allowed")
	// ErrUserBlocked is returned when a sender attempts to message a user who
	// has blocked them (or whom they have blocked) in an existing direct
	// conversation. Mapped to 403 Forbidden at the transport layer.
	ErrUserBlocked              = errors.New("user is blocked")
	ErrNotConversationMember    = errors.New("user is not a conversation member")
	ErrInsufficientChannelRole  = errors.New("insufficient channel role")
	ErrInviteLinkInvalid        = errors.New("invite link is invalid")
	ErrConversationMemberExists = errors.New("conversation member already exists")
	ErrCannotRemoveOwner        = errors.New("cannot remove conversation owner")
	ErrMessageNotScheduled      = errors.New("message is not scheduled")

	// Device / session errors added for session-device refactor.
	ErrDeviceNotFound = errors.New("device not found")
	ErrSessionExpired = errors.New("session has expired")
	ErrSessionRevoked = errors.New("session has been revoked")

	// Friendship errors
	ErrFriendshipNotFound     = errors.New("friendship not found")
	ErrFriendshipAlreadyExist = errors.New("friendship already exists or pending")
	ErrCannotFriendYourself   = errors.New("cannot send friendship request to yourself")
	ErrNotAddressee           = errors.New("only addressee can perform this action")
	ErrNotRequester           = errors.New("only requester can perform this action")
	ErrNotFriend              = errors.New("users are not friends")
	ErrCloseFriendNotFound    = errors.New("close friend not found")

	// Photos & Albums
	ErrPhotoNotFound        = errors.New("photo not found")
	ErrAlbumNotFound        = errors.New("album not found")
	ErrStorageQuotaExceeded = errors.New("storage quota exceeded")
	ErrPhotoAccessDenied    = errors.New("access to photo denied")
	ErrAlbumAccessDenied    = errors.New("access to album denied")
	ErrPhotoNotOwned        = errors.New("photo does not belong to requester")
	ErrAlbumNotOwned        = errors.New("album does not belong to requester")

	// Photo comments & likes
	ErrCommentNotFound          = errors.New("comment not found")
	ErrCommentNotOwned          = errors.New("comment does not belong to requester")
	ErrCommentNestingNotAllowed = errors.New("replies to replies are not allowed")
	ErrCommentEditExpired       = errors.New("comment edit window has expired")
	ErrCommentAlreadyDeleted    = errors.New("comment is already deleted")
	ErrPhotoAlreadyLiked        = errors.New("photo is already liked")
	ErrPhotoNotLiked            = errors.New("photo is not liked")
	ErrCommentAlreadyLiked      = errors.New("comment is already liked")
	ErrCommentNotLiked          = errors.New("comment is not liked")
)
