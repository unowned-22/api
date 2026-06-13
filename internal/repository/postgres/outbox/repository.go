package outboxrepo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	dom "github.com/unowned-22/api/internal/domain/outbox"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) dom.Repository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Insert(ctx context.Context, evt *dom.OutboxEvent) error {
	_, err := r.pool.Exec(ctx, `
        INSERT INTO outbox_events (id, event_type, payload, status, created_at, retry_count)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, evt.ID, evt.EventType, evt.Payload, dom.StatusPending, evt.CreatedAt, evt.RetryCount)
	if err != nil {
		return fmt.Errorf("outbox insert failed: %w", err)
	}
	return nil
}

// FetchAndMarkProcessing atomically selects pending events and marks them as processing.
func (r *PostgresRepository) FetchAndMarkProcessing(ctx context.Context, limit int) ([]*dom.OutboxEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	rows, err := tx.Query(ctx, `
        SELECT id, event_type, payload, status, created_at, processed_at, retry_count
        FROM outbox_events
        WHERE status = $1
        ORDER BY created_at
        FOR UPDATE SKIP LOCKED
        LIMIT $2
    `, dom.StatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("select pending: %w", err)
	}

	var events []*dom.OutboxEvent
	for rows.Next() {
		var e dom.OutboxEvent
		var processedAt *time.Time
		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload, &e.Status, &e.CreatedAt, &processedAt, &e.RetryCount); err != nil {
			return nil, fmt.Errorf("scan outbox: %w", err)
		}
		e.ProcessedAt = processedAt
		events = append(events, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}

	// mark as processing
	ids := make([]interface{}, 0, len(events))
	for i, ev := range events {
		ids = append(ids, ev.ID)
		// update each row
		if _, err := tx.Exec(ctx, `UPDATE outbox_events SET status = $1 WHERE id = $2`, dom.StatusProcessing, ev.ID); err != nil {
			return nil, fmt.Errorf("mark processing: %w", err)
		}
		// reflect change locally
		events[i].Status = dom.StatusProcessing
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit mark processing: %w", err)
	}

	return events, nil
}

func (r *PostgresRepository) MarkProcessed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE outbox_events SET status = $1, processed_at = $2 WHERE id = $3`, dom.StatusProcessed, time.Now(), id)
	if err != nil {
		return fmt.Errorf("mark processed: %w", err)
	}
	return nil
}

func (r *PostgresRepository) IncrementRetry(ctx context.Context, id string) (int, error) {
	var retry int
	err := r.pool.QueryRow(ctx, `UPDATE outbox_events SET retry_count = retry_count + 1 WHERE id = $1 RETURNING retry_count`, id).Scan(&retry)
	if err != nil {
		return 0, fmt.Errorf("increment retry: %w", err)
	}
	return retry, nil
}

func (r *PostgresRepository) MarkFailed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE outbox_events SET status = $1 WHERE id = $2`, dom.StatusFailed, id)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	return nil
}
