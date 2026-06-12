package event

import "context"

// Name — strongly typed event name.
type Name string

const (
	UserRegistered         Name = "user.registered"
	PasswordResetRequested Name = "password.reset.requested"
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
