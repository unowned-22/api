package outbox

import (
	"context"
	"time"

	"github.com/google/uuid"
	dom "github.com/unowned-22/api/internal/domain/outbox"
)

// PayloadEvent is an event that carries a payload and a name.
type PayloadEvent interface {
	EventName() string
	Payload() []byte
}

// NewBridgeHandler returns a handler function suitable for subscribing to an in-process bus.
// It will insert incoming events into the outbox repository for later delivery.
func NewBridgeHandler(repo dom.Repository) func(ctx context.Context, e PayloadEvent) error {
	return func(ctx context.Context, e PayloadEvent) error {
		evt := &dom.OutboxEvent{
			ID:         uuid.NewString(),
			EventType:  e.EventName(),
			Payload:    e.Payload(),
			Status:     dom.StatusPending,
			CreatedAt:  time.Now(),
			RetryCount: 0,
		}
		return repo.Insert(ctx, evt)
	}
}
