package handler

import (
	"context"
	"fmt"

	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/logger"
)

// MultiHandler runs several handlers for the same event name in sequence.
//
// The AMQP consumer dispatches exactly one event.Handler per event.Name
// (see infrastructure/queue.AMQPConsumer), so whenever a second, independent
// concern needs to react to an event that already has a handler (e.g.
// audit logging + search indexing on the same event), wrap them with
// MultiHandler instead of adding unrelated logic to an existing handler.
//
// All sub-handlers run even if one fails, so an indexing failure cannot
// suppress, e.g., audit persistence. The first error encountered is
// returned (after all handlers ran) so the broker still nacks/retries the
// message; the rest are logged.
type MultiHandler struct {
	name     event.Name
	handlers []event.Handler
}

// NewMultiHandler creates a fan-out handler for evt that invokes each of
// handlers in order.
func NewMultiHandler(evt event.Name, handlers ...event.Handler) *MultiHandler {
	return &MultiHandler{name: evt, handlers: handlers}
}

func (h *MultiHandler) EventName() event.Name {
	return h.name
}

func (h *MultiHandler) Handle(ctx context.Context, payload []byte) error {
	var firstErr error
	for _, sub := range h.handlers {
		if sub == nil {
			continue
		}
		if err := sub.Handle(ctx, payload); err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{
				"event": string(h.name),
			}).Error("multi_handler: sub-handler failed")
			if firstErr == nil {
				firstErr = fmt.Errorf("multi_handler(%s): %w", h.name, err)
			}
		}
	}
	return firstErr
}

var _ event.Handler = (*MultiHandler)(nil)
