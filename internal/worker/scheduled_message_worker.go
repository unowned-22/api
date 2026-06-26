package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/messenger"
	"github.com/unowned-22/api/internal/logger"
)

// ScheduledMessageWorker polls for due scheduled messages every 30 seconds and
// publishes them through the outbox so the realtime consumer picks them up.
type ScheduledMessageWorker struct {
	msgRepo    messenger.MessageRepository
	memberRepo messenger.MemberRepository
	eventBus   event.Publisher
}

func NewScheduledMessageWorker(msgRepo messenger.MessageRepository, memberRepo messenger.MemberRepository, eventBus event.Publisher) *ScheduledMessageWorker {
	return &ScheduledMessageWorker{msgRepo: msgRepo, memberRepo: memberRepo, eventBus: eventBus}
}

func (w *ScheduledMessageWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processDue(ctx)
		}
	}
}

func (w *ScheduledMessageWorker) processDue(ctx context.Context) {
	msgs, err := w.msgRepo.ListDueScheduled(ctx)
	if err != nil {
		logger.Log.WithError(err).Error("ScheduledMessageWorker: failed to list due scheduled messages")
		return
	}

	for _, m := range msgs {
		if err := w.msgRepo.MarkScheduledSent(ctx, m.ID); err != nil {
			logger.Log.WithError(err).Errorf("ScheduledMessageWorker: failed to mark message %d as sent", m.ID)
			continue
		}

		members, err := w.memberRepo.ListMembers(ctx, m.ConversationID)
		if err != nil {
			logger.Log.WithError(err).Errorf("ScheduledMessageWorker: failed to list members for conv %d", m.ConversationID)
			continue
		}
		recipientIDs := make([]int64, 0, len(members))
		for _, mem := range members {
			recipientIDs = append(recipientIDs, mem.UserID)
		}

		payload, err := json.Marshal(map[string]interface{}{
			"conversation_id": m.ConversationID,
			"message_id":      m.ID,
			"sender_id":       m.SenderID,
			"recipient_ids":   recipientIDs,
		})
		if err != nil {
			logger.Log.WithError(err).Errorf("ScheduledMessageWorker: failed to marshal payload for message %d", m.ID)
			continue
		}

		if err := w.eventBus.Publish(ctx, event.Event{
			Name:    event.MessengerScheduledReady,
			Payload: payload,
		}); err != nil {
			logger.Log.WithError(err).Errorf("ScheduledMessageWorker: failed to publish event for message %d", m.ID)
		}
	}
}

// compile-time check
var _ Worker = (*ScheduledMessageWorker)(nil)
