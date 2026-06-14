package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	domaev "github.com/unowned-22/api/internal/domain/event"
	dom "github.com/unowned-22/api/internal/domain/outbox"
	"github.com/unowned-22/api/internal/logger"
)

type OutboxPublisher struct {
	repo dom.Repository
}

func New(repo dom.Repository) *OutboxPublisher {
	return &OutboxPublisher{repo: repo}
}

func (p *OutboxPublisher) Publish(ctx context.Context, evt domaev.Event) error {
	out := &dom.OutboxEvent{
		ID:         uuid.NewString(),
		EventType:  string(evt.Name),
		Payload:    evt.Payload,
		Status:     dom.StatusPending,
		CreatedAt:  time.Now(),
		RetryCount: 0,
	}
	if err := p.repo.Insert(ctx, out); err != nil {
		logger.Log.WithError(err).WithField("event", evt.Name).Error("outbox publisher: failed to insert event")
		return err
	}
	logger.Log.WithField("event", evt.Name).Debug("outbox publisher: event inserted")
	return nil
}

func (p *OutboxPublisher) PublishTx(ctx context.Context, tx pgx.Tx, evt domaev.Event) error {
	out := &dom.OutboxEvent{
		ID:         uuid.NewString(),
		EventType:  string(evt.Name),
		Payload:    evt.Payload,
		Status:     dom.StatusPending,
		CreatedAt:  time.Now(),
		RetryCount: 0,
	}

	txRepo, ok := p.repo.(interface {
		InsertTx(ctx context.Context, tx pgx.Tx, evt *dom.OutboxEvent) error
	})
	if !ok {
		return fmt.Errorf("outbox repository does not support transactions")
	}

	if err := txRepo.InsertTx(ctx, tx, out); err != nil {
		logger.Log.WithError(err).WithField("event", evt.Name).Error("outbox publisher: failed to insert event in tx")
		return err
	}
	logger.Log.WithField("event", evt.Name).Debug("outbox publisher: event inserted in tx")
	return nil
}

func (p *OutboxPublisher) Close() error { return nil }
