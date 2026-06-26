package worker

import (
	"context"
	"time"

	"github.com/unowned-22/api/internal/domain/messenger"
	"github.com/unowned-22/api/internal/logger"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

// DisappearingMessageWorker polls every minute for messages whose disappears_at
// has passed, hard-deletes them from the DB, and notifies conversation members
// via the WebSocket hub.
type DisappearingMessageWorker struct {
	msgRepo    messenger.MessageRepository
	memberRepo messenger.MemberRepository
	hub        *ws.Hub
}

func NewDisappearingMessageWorker(msgRepo messenger.MessageRepository, memberRepo messenger.MemberRepository, hub *ws.Hub) *DisappearingMessageWorker {
	return &DisappearingMessageWorker{msgRepo: msgRepo, memberRepo: memberRepo, hub: hub}
}

func (w *DisappearingMessageWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processExpired(ctx)
		}
	}
}

func (w *DisappearingMessageWorker) processExpired(ctx context.Context) {
	msgs, err := w.msgRepo.ListExpiredDisappearing(ctx)
	if err != nil {
		logger.Log.WithError(err).Error("DisappearingMessageWorker: failed to list expired disappearing messages")
		return
	}

	// membersCache avoids N redundant ListMembers queries when multiple messages
	// in the same conversation expire in one ticker pass (common when a group
	// sets a uniform disappear timer and sends messages in a burst).
	membersCache := make(map[int64][]*messenger.ConversationMember)

	for _, m := range msgs {
		if err := w.msgRepo.HardDeleteByID(ctx, m.ID); err != nil {
			logger.Log.WithError(err).Errorf("DisappearingMessageWorker: failed to hard-delete message %d", m.ID)
			continue
		}

		members, ok := membersCache[m.ConversationID]
		if !ok {
			members, err = w.memberRepo.ListMembers(ctx, m.ConversationID)
			if err != nil {
				logger.Log.WithError(err).Errorf("DisappearingMessageWorker: failed to list members for conversation %d", m.ConversationID)
				continue
			}
			membersCache[m.ConversationID] = members
		}

		for _, member := range members {
			if err := ws.SendMessengerEvent(w.hub, member.UserID, "messenger.message_deleted",
				ws.MessengerMessageDeletedPayload{ConversationID: m.ConversationID, MessageID: m.ID}); err != nil {
				logger.Log.WithError(err).Warnf("DisappearingMessageWorker: failed to push WS event to user %d", member.UserID)
			}
		}

		logger.Log.Debugf("DisappearingMessageWorker: hard-deleted message %d and notified %d members", m.ID, len(members))
	}
}

// compile-time check
var _ Worker = (*DisappearingMessageWorker)(nil)
