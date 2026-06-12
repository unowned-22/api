package event

import "context"

// Handler defines the contract for processing a specific event type.
// Each concrete handler implements this interface for a particular event.
type Handler interface {
	// EventName returns the name of the event this handler processes.
	EventName() Name

	// Handle processes the event payload.
	// Return error will trigger Nack(requeue=false) on the broker.
	Handle(ctx context.Context, payload []byte) error
}
