package event

import "context"

// Name — strongly typed event name.
type Name string

const (
	UserRegistered              Name = "user.registered"
	PasswordResetRequested      Name = "password.reset.requested"
	LoginSuccess                Name = "audit.login_success"
	LoginFailed                 Name = "audit.login_failed"
	Logout                      Name = "audit.logout"
	LogoutAll                   Name = "audit.logout_all"
	VerificationSent            Name = "audit.verification_sent"
	EmailVerified               Name = "audit.email_verified"
	PasswordResetCompleted      Name = "audit.password_reset_completed"
	PasswordResetRequestedAudit Name = "audit.password_reset_requested"
	PasswordChanged             Name = "audit.password_changed"
	RefreshRotated              Name = "audit.refresh_rotated"
	SessionRevoked              Name = "audit.session_revoked"
	AccountDeactivated          Name = "audit.account_deactivated"
	AccountActivated            Name = "audit.account_activated"
	RefreshTokenReuseDetected   Name = "audit.refresh_token_reuse_detected"
	// UserEmailVerified is published after successful email verification and triggers
	// provisioning of the user bucket and user_settings. It is distinct from the
	// audit.email_verified event so that AuditHandler and provisioning can subscribe
	// independently.
	UserEmailVerified Name = "user.email_verified"
	// EmailSend is used to request an email send via the outbox/worker pipeline.
	EmailSend Name = "email.send"

	// Friend events
	FriendRequestReceived Name = "friend.request_received"
	FriendRequestAccepted Name = "friend.request_accepted"
)

// Event is a unit of publication.
type Event struct {
	Name    Name
	Payload []byte // JSON-serialized body
}

// Publisher is the contract for publishing events.
// Implementation lives in internal/infrastructure/queue.
type Publisher interface {
	Publish(ctx context.Context, event Event) error
	Close() error
}
