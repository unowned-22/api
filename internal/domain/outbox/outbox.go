package outbox

import (
	"context"
	"time"
)

// Status values for outbox events
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusProcessed  = "processed"
	StatusFailed     = "failed"
)

// OutboxEvent represents an event stored in the outbox table.
type OutboxEvent struct {
	ID          string     `json:"id"`
	EventType   string     `json:"event_type"`
	Payload     []byte     `json:"payload"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ProcessedAt *time.Time `json:"processed_at"`
	RetryCount  int        `json:"retry_count"`
}

// Repository defines storage operations for outbox events.
type Repository interface {
	Insert(ctx context.Context, evt *OutboxEvent) error
	FetchAndMarkProcessing(ctx context.Context, limit int) ([]*OutboxEvent, error)
	MarkProcessed(ctx context.Context, id string) error
	IncrementRetry(ctx context.Context, id string) (int, error)
	MarkFailed(ctx context.Context, id string) error
}
