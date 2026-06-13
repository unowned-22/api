package eventbus

import "context"

// Event is a domain event message.
type Event interface {
	EventName() string
}

// Handler handles domain events.
type Handler interface {
	Handle(ctx context.Context, event Event) error
}

// Bus publishes events to subscribed handlers.
type Bus interface {
	Publish(ctx context.Context, event Event) error

	Subscribe(
		eventName string,
		handler Handler,
	)
}
