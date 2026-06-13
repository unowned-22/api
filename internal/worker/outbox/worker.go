package outbox

import (
	"context"
	"time"

	domaev "github.com/unowned-22/api/internal/domain/event"
	repo "github.com/unowned-22/api/internal/domain/outbox"
	"github.com/unowned-22/api/internal/logger"
)

type RetryPolicy struct {
	MaxRetries int
	Interval   time.Duration // poll interval
}

type OutboxWorker struct {
	repo   repo.Repository
	pub    domaev.Publisher
	policy RetryPolicy
	limit  int
}

func NewWorker(r repo.Repository, pub domaev.Publisher, policy RetryPolicy, limit int) *OutboxWorker {
	return &OutboxWorker{repo: r, pub: pub, policy: policy, limit: limit}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.policy.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Log.Info("outbox worker: shutdown requested")
			return
		default:
		}

		events, err := w.repo.FetchAndMarkProcessing(ctx, w.limit)
		if err != nil {
			logger.Log.WithError(err).Error("outbox: failed fetching events")
			select {
			case <-ctx.Done():
				return
			case <-time.After(w.policy.Interval):
				continue
			}
		}

		for _, ev := range events {
			// publish to RabbitMQ
			err := w.pub.Publish(ctx, domaev.Event{Name: domaev.Name(ev.EventType), Payload: ev.Payload})
			if err != nil {
				logger.Log.WithFields(map[string]interface{}{"event_id": ev.ID, "event_type": ev.EventType}).WithError(err).Warn("outbox: publish failed")
				retries, incErr := w.repo.IncrementRetry(ctx, ev.ID)
				if incErr != nil {
					logger.Log.WithError(incErr).WithField("event_id", ev.ID).Error("outbox: failed to increment retry")
				}
				if retries >= w.policy.MaxRetries {
					if err := w.repo.MarkFailed(ctx, ev.ID); err != nil {
						logger.Log.WithError(err).WithField("event_id", ev.ID).Error("outbox: failed to mark failed")
					}
				}
				continue
			}

			if err := w.repo.MarkProcessed(ctx, ev.ID); err != nil {
				logger.Log.WithError(err).WithField("event_id", ev.ID).Error("outbox: failed to mark processed")
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			continue
		}
	}
}
