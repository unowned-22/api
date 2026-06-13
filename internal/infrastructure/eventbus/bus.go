package inmemory

import (
	"context"
	"reflect"
	"sync"

	dom "github.com/unowned-22/api/internal/domain/eventbus"
	"github.com/unowned-22/api/internal/logger"
)

// InMemoryBus is a simple, thread-safe in-process event bus.
// It delivers events to all subscribed handlers asynchronously.
type InMemoryBus struct {
	mu   sync.RWMutex
	subs map[string][]dom.Handler
}

// NewInMemoryBus creates a new InMemoryBus instance.
func NewInMemoryBus() dom.Bus {
	return &InMemoryBus{
		subs: make(map[string][]dom.Handler),
	}
}

// Subscribe registers a handler for an event name.
func (b *InMemoryBus) Subscribe(eventName string, handler dom.Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[eventName] = append(b.subs[eventName], handler)
	logger.Log.WithField("event", eventName).Info("eventbus: handler subscribed")
}

// Publish dispatches the event to all subscribers without blocking the caller.
// Each handler is invoked in its own goroutine. Panics and errors from handlers
// are recovered and logged; they do not affect other handlers.
func (b *InMemoryBus) Publish(ctx context.Context, event dom.Event) error {
	b.mu.RLock()
	handlers := append([]dom.Handler(nil), b.subs[event.EventName()]...)
	b.mu.RUnlock()

	if len(handlers) == 0 {
		logger.Log.WithField("event", event.EventName()).Debug("eventbus: no handlers for event")
		return nil
	}

	for _, h := range handlers {
		handler := h
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Log.WithFields(map[string]interface{}{
						"event":   event.EventName(),
						"handler": reflect.TypeOf(handler).String(),
						"panic":   r,
					}).Error("eventbus: handler panic recovered")
				}
			}()

			if err := handler.Handle(ctx, event); err != nil {
				logger.Log.WithFields(map[string]interface{}{
					"event":   event.EventName(),
					"handler": reflect.TypeOf(handler).String(),
					"error":   err.Error(),
				}).Error("eventbus: handler returned error")
			}
		}()
	}

	return nil
}
