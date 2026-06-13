package event

import "context"

// Name — strongly typed event name.
type Name string

const (
	UserRegistered         Name = "user.registered"
	PasswordResetRequested Name = "password.reset.requested"
	// Audit / security events
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
