package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/messenger"
	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/domain/usersettings"
	"github.com/unowned-22/api/internal/logger"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

// messengerBroadcaster is a helper that fans out a WS event to all active
// members of a conversation.
type messengerBroadcaster struct {
	memberRepo messenger.MemberRepository
	hub        *ws.Hub
}

func (b *messengerBroadcaster) broadcast(ctx context.Context, convID int64, wsType string, payload any) error {
	members, err := b.memberRepo.ListMembers(ctx, convID)
	if err != nil {
		return fmt.Errorf("failed to list members for conv %d: %w", convID, err)
	}
	for _, m := range members {
		if err := ws.SendMessengerEvent(b.hub, m.UserID, wsType, payload); err != nil {
			logger.Log.WithError(err).Warnf("messenger realtime: failed to push to user %d", m.UserID)
		}
	}
	return nil
}

type MessengerMessageSentHandler struct {
	hub         *ws.Hub
	usersetRepo usersettings.Repository
	notifRepo   notification.Repository
}

func NewMessengerMessageSentHandler(hub *ws.Hub, usersetRepo usersettings.Repository, notifRepo notification.Repository) *MessengerMessageSentHandler {
	return &MessengerMessageSentHandler{hub: hub, usersetRepo: usersetRepo, notifRepo: notifRepo}
}

func (h *MessengerMessageSentHandler) EventName() event.Name { return event.MessengerMessageSent }

func (h *MessengerMessageSentHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("MessengerMessageSent event received")
	var p struct {
		ConversationID int64              `json:"conversation_id"`
		Message        *messenger.Message `json:"message"`
		RecipientIDs   []int64            `json:"recipient_ids"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	wsPayload := ws.MessengerMessagePayload{
		ConversationID: p.ConversationID,
		Message:        p.Message,
	}

	var offlineNotifs []*notification.Notification
	mentionSet := make(map[int64]bool, len(p.Message.MentionUserIDs))
	for _, uid := range p.Message.MentionUserIDs {
		mentionSet[uid] = true
	}

	for _, uid := range p.RecipientIDs {
		if err := ws.SendMessengerEvent(h.hub, uid, "messenger.message_sent", wsPayload); err != nil {
			logger.Log.WithError(err).Warnf("messenger: failed to push to user %d", uid)
		}

		if uid == p.Message.SenderID {
			continue
		}
		if h.hub.HasUser(uid) {
			continue
		}

		notifType := notification.TypeMessengerNewMessage
		if mentionSet[uid] {
			notifType = notification.TypeMessengerMentioned
		}

		prefs, err := h.usersetRepo.GetNotificationPreferences(ctx, uid)
		if err != nil {
			logger.Log.WithError(err).Warnf("messenger: failed to load prefs for user %d", uid)
			continue
		}
		allow := true
		if len(prefs) > 0 {
			var mp map[string]bool
			if err := json.Unmarshal(prefs, &mp); err == nil {
				if v, ok := mp[string(notifType)]; ok {
					allow = v
				}
			}
		}
		if !allow {
			continue
		}

		notifPayload, _ := json.Marshal(map[string]any{
			"conversation_id": p.ConversationID,
			"message_id":      p.Message.ID,
			"sender_id":       p.Message.SenderID,
		})
		offlineNotifs = append(offlineNotifs, &notification.Notification{
			UserID:     uid,
			ActorID:    p.Message.SenderID,
			Type:       notifType,
			EntityType: "message",
			EntityID:   p.Message.ID,
			Payload:    notifPayload,
			IsRead:     false,
			CreatedAt:  time.Now(),
		})
	}

	if len(offlineNotifs) > 0 {
		if err := h.notifRepo.CreateBatch(ctx, offlineNotifs); err != nil {
			logger.Log.WithError(err).Warn("messenger: failed to create offline notifications")
		}
	}
	return nil
}

type MessengerScheduledReadyHandler struct {
	hub *ws.Hub
}

func NewMessengerScheduledReadyHandler(hub *ws.Hub) *MessengerScheduledReadyHandler {
	return &MessengerScheduledReadyHandler{hub: hub}
}

func (h *MessengerScheduledReadyHandler) EventName() event.Name { return event.MessengerScheduledReady }

func (h *MessengerScheduledReadyHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("MessengerScheduledReady event received")
	var p struct {
		ConversationID int64   `json:"conversation_id"`
		MessageID      int64   `json:"message_id"`
		SenderID       int64   `json:"sender_id"`
		RecipientIDs   []int64 `json:"recipient_ids"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	wsPayload := struct {
		ConversationID int64 `json:"conversation_id"`
		MessageID      int64 `json:"message_id"`
		SenderID       int64 `json:"sender_id"`
	}{
		ConversationID: p.ConversationID,
		MessageID:      p.MessageID,
		SenderID:       p.SenderID,
	}
	for _, uid := range p.RecipientIDs {
		if err := ws.SendMessengerEvent(h.hub, uid, "messenger.message_sent", wsPayload); err != nil {
			logger.Log.WithError(err).Warnf("messenger: failed to push to user %d", uid)
		}
	}
	return nil
}

type MessengerReactionHandler struct {
	messengerBroadcaster
	evtName event.Name
}

func NewMessengerReactionHandler(evtName event.Name, memberRepo messenger.MemberRepository, hub *ws.Hub) *MessengerReactionHandler {
	return &MessengerReactionHandler{messengerBroadcaster: messengerBroadcaster{memberRepo: memberRepo, hub: hub}, evtName: evtName}
}

func (h *MessengerReactionHandler) EventName() event.Name { return h.evtName }

func (h *MessengerReactionHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Infof("%s event received", h.evtName)
	var p ws.MessengerReactionPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	wsType := "messenger.reaction_added"
	if h.evtName == event.MessengerReactionRemoved {
		wsType = "messenger.reaction_removed"
	}
	return h.broadcast(ctx, p.ConversationID, wsType, p)
}

type MessengerDeliveryUpdatedHandler struct {
	hub *ws.Hub
}

func NewMessengerDeliveryUpdatedHandler(hub *ws.Hub) *MessengerDeliveryUpdatedHandler {
	return &MessengerDeliveryUpdatedHandler{hub: hub}
}

func (h *MessengerDeliveryUpdatedHandler) EventName() event.Name {
	return event.MessengerDeliveryUpdated
}

func (h *MessengerDeliveryUpdatedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("MessengerDeliveryUpdated event received")
	var p ws.MessengerDeliveryPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	return ws.SendMessengerEvent(h.hub, p.UserID, "messenger.delivery_updated", p)
}

type MessengerPinHandler struct {
	messengerBroadcaster
	evtName event.Name
}

func NewMessengerPinHandler(evtName event.Name, memberRepo messenger.MemberRepository, hub *ws.Hub) *MessengerPinHandler {
	return &MessengerPinHandler{messengerBroadcaster: messengerBroadcaster{memberRepo: memberRepo, hub: hub}, evtName: evtName}
}

func (h *MessengerPinHandler) EventName() event.Name { return h.evtName }

func (h *MessengerPinHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Infof("%s event received", h.evtName)
	var p ws.MessengerPinPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	wsType := "messenger.message_pinned"
	if h.evtName == event.MessengerMessageUnpinned {
		wsType = "messenger.message_unpinned"
	}
	return h.broadcast(ctx, p.ConversationID, wsType, p)
}

type MessengerMessageEditedHandler struct {
	messengerBroadcaster
}

func NewMessengerMessageEditedHandler(memberRepo messenger.MemberRepository, hub *ws.Hub) *MessengerMessageEditedHandler {
	return &MessengerMessageEditedHandler{messengerBroadcaster: messengerBroadcaster{memberRepo: memberRepo, hub: hub}}
}

func (h *MessengerMessageEditedHandler) EventName() event.Name { return event.MessengerMessageEdited }

func (h *MessengerMessageEditedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("MessengerMessageEdited event received")
	var p struct {
		ConversationID int64     `json:"conversation_id"`
		MessageID      int64     `json:"message_id"`
		NewBody        string    `json:"new_body"`
		EditedAt       time.Time `json:"edited_at"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	return h.broadcast(ctx, p.ConversationID, "messenger.message_edited", p)
}

type MessengerMessageDeletedHandler struct {
	messengerBroadcaster
}

func NewMessengerMessageDeletedHandler(memberRepo messenger.MemberRepository, hub *ws.Hub) *MessengerMessageDeletedHandler {
	return &MessengerMessageDeletedHandler{messengerBroadcaster: messengerBroadcaster{memberRepo: memberRepo, hub: hub}}
}

func (h *MessengerMessageDeletedHandler) EventName() event.Name { return event.MessengerMessageDeleted }

func (h *MessengerMessageDeletedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("MessengerMessageDeleted event received")
	var p ws.MessengerMessageDeletedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	return h.broadcast(ctx, p.ConversationID, "messenger.message_deleted", p)
}

type MessengerReadReceiptHandler struct {
	messengerBroadcaster
}

func NewMessengerReadReceiptHandler(memberRepo messenger.MemberRepository, hub *ws.Hub) *MessengerReadReceiptHandler {
	return &MessengerReadReceiptHandler{messengerBroadcaster: messengerBroadcaster{memberRepo: memberRepo, hub: hub}}
}

func (h *MessengerReadReceiptHandler) EventName() event.Name { return event.MessengerReadReceipt }

func (h *MessengerReadReceiptHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("MessengerReadReceipt event received")
	var p ws.MessengerReadPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	return h.broadcast(ctx, p.ConversationID, "messenger.read", p)
}

type MessengerMemberAddedHandler struct {
	messengerBroadcaster
}

func NewMessengerMemberAddedHandler(memberRepo messenger.MemberRepository, hub *ws.Hub) *MessengerMemberAddedHandler {
	return &MessengerMemberAddedHandler{messengerBroadcaster: messengerBroadcaster{memberRepo: memberRepo, hub: hub}}
}

func (h *MessengerMemberAddedHandler) EventName() event.Name { return event.MessengerMemberAdded }

func (h *MessengerMemberAddedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("MessengerMemberAdded event received")
	var p struct {
		ConversationID int64  `json:"conversation_id"`
		UserID         int64  `json:"user_id"`
		Role           string `json:"role"`
		ActorID        int64  `json:"actor_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	return h.broadcast(ctx, p.ConversationID, "messenger.member_added", p)
}

type MessengerMemberRemovedHandler struct {
	messengerBroadcaster
}

func NewMessengerMemberRemovedHandler(memberRepo messenger.MemberRepository, hub *ws.Hub) *MessengerMemberRemovedHandler {
	return &MessengerMemberRemovedHandler{messengerBroadcaster: messengerBroadcaster{memberRepo: memberRepo, hub: hub}}
}

func (h *MessengerMemberRemovedHandler) EventName() event.Name { return event.MessengerMemberRemoved }

func (h *MessengerMemberRemovedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("MessengerMemberRemoved event received")
	var p struct {
		ConversationID int64 `json:"conversation_id"`
		UserID         int64 `json:"user_id"`
		ActorID        int64 `json:"actor_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	return h.broadcast(ctx, p.ConversationID, "messenger.member_removed", p)
}

type MessengerTypingHandler struct {
	messengerBroadcaster
}

func NewMessengerTypingHandler(memberRepo messenger.MemberRepository, hub *ws.Hub) *MessengerTypingHandler {
	return &MessengerTypingHandler{messengerBroadcaster: messengerBroadcaster{memberRepo: memberRepo, hub: hub}}
}

func (h *MessengerTypingHandler) EventName() event.Name { return event.MessengerTyping }

func (h *MessengerTypingHandler) Handle(ctx context.Context, payload []byte) error {
	var p ws.MessengerTypingPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	members, err := h.memberRepo.ListMembers(ctx, p.ConversationID)
	if err != nil {
		return fmt.Errorf("list members: %w", err)
	}
	for _, m := range members {
		if m.UserID == p.UserID {
			continue
		}
		if err := ws.SendMessengerEvent(h.hub, m.UserID, "messenger.typing", p); err != nil {
			logger.Log.WithError(err).Warnf("typing: failed to push to user %d", m.UserID)
		}
	}
	return nil
}

var (
	_ event.Handler = (*MessengerMessageSentHandler)(nil)
	_ event.Handler = (*MessengerScheduledReadyHandler)(nil)
	_ event.Handler = (*MessengerReactionHandler)(nil)
	_ event.Handler = (*MessengerDeliveryUpdatedHandler)(nil)
	_ event.Handler = (*MessengerPinHandler)(nil)
	_ event.Handler = (*MessengerMessageEditedHandler)(nil)
	_ event.Handler = (*MessengerMessageDeletedHandler)(nil)
	_ event.Handler = (*MessengerReadReceiptHandler)(nil)
	_ event.Handler = (*MessengerMemberAddedHandler)(nil)
	_ event.Handler = (*MessengerMemberRemovedHandler)(nil)
	_ event.Handler = (*MessengerTypingHandler)(nil)
)
