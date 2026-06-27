package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/domain/messenger"
	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/domain/story"
	"github.com/unowned-22/api/internal/domain/usersettings"
	"github.com/unowned-22/api/internal/infrastructure/queue"
	"github.com/unowned-22/api/internal/logger"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

type Consumer struct {
	consumer *queue.AMQPConsumer
}

func NewConsumer(cfg *config.Config, friendshipRepo friendship.Repository, storyRepo story.StoryRepository, userSettingsRepo usersettings.Repository, notificationRepo notification.Repository, hub *ws.Hub, messengerMemberRepo messenger.MemberRepository) (*Consumer, error) {
	handlers := map[event.Name]event.Handler{
		event.FriendRequestReceived: NewFriendRequestReceivedHandler(userSettingsRepo, notificationRepo, hub),
		event.FriendRequestAccepted: NewFriendRequestAcceptedHandler(userSettingsRepo, notificationRepo, hub),
		event.StoryPublished:        NewStoryPublishedHandler(friendshipRepo, storyRepo, userSettingsRepo, notificationRepo, hub),
		event.PhotoLiked:            NewPhotoLikedHandler(userSettingsRepo, notificationRepo, hub),
		event.PhotoCommented:        NewPhotoCommentedHandler(userSettingsRepo, notificationRepo, hub),
		event.CommentReplied:        NewCommentRepliedHandler(userSettingsRepo, notificationRepo, hub),
		event.CommentLiked:          NewCommentLikedHandler(userSettingsRepo, notificationRepo, hub),
		// Messenger realtime events
		event.MessengerMessageSent:     NewMessengerMessageSentHandler(hub, userSettingsRepo, notificationRepo),
		event.MessengerScheduledReady:  NewMessengerScheduledReadyHandler(hub),
		event.MessengerReactionAdded:   NewMessengerReactionHandler(event.MessengerReactionAdded, messengerMemberRepo, hub),
		event.MessengerReactionRemoved: NewMessengerReactionHandler(event.MessengerReactionRemoved, messengerMemberRepo, hub),
		event.MessengerDeliveryUpdated: NewMessengerDeliveryUpdatedHandler(hub),
		event.MessengerMessagePinned:   NewMessengerPinHandler(event.MessengerMessagePinned, messengerMemberRepo, hub),
		event.MessengerMessageUnpinned: NewMessengerPinHandler(event.MessengerMessageUnpinned, messengerMemberRepo, hub),
		event.MessengerMessageEdited:   NewMessengerMessageEditedHandler(messengerMemberRepo, hub),
		event.MessengerMessageDeleted:  NewMessengerMessageDeletedHandler(messengerMemberRepo, hub),
		event.MessengerReadReceipt:     NewMessengerReadReceiptHandler(messengerMemberRepo, hub),
		event.MessengerMemberAdded:     NewMessengerMemberAddedHandler(messengerMemberRepo, hub),
		event.MessengerMemberRemoved:   NewMessengerMemberRemovedHandler(messengerMemberRepo, hub),
		event.MessengerTyping:          NewMessengerTypingHandler(messengerMemberRepo, hub),
	}

	consumer, err := queue.NewConsumer(queue.ConsumerConfig{
		URL:                  cfg.RabbitMQURL,
		Exchange:             cfg.RabbitMQExchange,
		Queue:                cfg.RabbitMQRealtimeQueue,
		Tag:                  "serve-realtime",
		DeadLetterExchange:   cfg.RabbitMQDeadLetterExchange,
		DeadLetterRoutingKey: cfg.RabbitMQRealtimeDeadLetterRoutingKey,
	}, handlers)
	if err != nil {
		return nil, fmt.Errorf("failed to create realtime AMQP consumer: %w", err)
	}

	return &Consumer{consumer: consumer}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	logger.Log.Info("Realtime consumer started")
	if err := c.consumer.Consume(); err != nil {
		return fmt.Errorf("failed to start realtime consumer: %w", err)
	}

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.consumer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown realtime consumer: %w", err)
	}

	return nil
}

func (c *Consumer) Shutdown(ctx context.Context) error {
	return c.consumer.Shutdown(ctx)
}

type FriendRequestReceivedHandler struct {
	usersetRepo usersettings.Repository
	notifRepo   notification.Repository
	hub         *ws.Hub
}

func NewFriendRequestReceivedHandler(u usersettings.Repository, r notification.Repository, h *ws.Hub) *FriendRequestReceivedHandler {
	return &FriendRequestReceivedHandler{usersetRepo: u, notifRepo: r, hub: h}
}

func (h *FriendRequestReceivedHandler) EventName() event.Name { return event.FriendRequestReceived }

func (h *FriendRequestReceivedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("FriendRequestReceived event received")

	var p struct {
		FriendshipID int64 `json:"friendship_id"`
		RequesterID  int64 `json:"requester_id"`
		AddresseeID  int64 `json:"addressee_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	prefs, err := h.usersetRepo.GetNotificationPreferences(ctx, p.AddresseeID)
	if err != nil {
		return fmt.Errorf("failed to load prefs: %w", err)
	}
	allow := true
	if prefs != nil && len(prefs) > 0 {
		var mp map[string]bool
		if err := json.Unmarshal(prefs, &mp); err == nil {
			if v, ok := mp[string(notification.TypeFriendRequestReceived)]; ok {
				allow = v
			}
		}
	}
	if !allow {
		return nil
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{"friendship_id": p.FriendshipID, "requester_id": p.RequesterID})
	n := &notification.Notification{
		UserID:     p.AddresseeID,
		ActorID:    p.RequesterID,
		Type:       notification.TypeFriendRequestReceived,
		EntityType: "friend_request",
		EntityID:   p.FriendshipID,
		Payload:    payloadBytes,
		IsRead:     false,
		CreatedAt:  time.Now(),
	}
	if err := h.notifRepo.Create(ctx, n); err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	if err := ws.SendNotification(h.hub, n.UserID, n); err != nil {
		return err
	}
	return nil
}

type FriendRequestAcceptedHandler struct {
	usersetRepo usersettings.Repository
	notifRepo   notification.Repository
	hub         *ws.Hub
}

func NewFriendRequestAcceptedHandler(u usersettings.Repository, r notification.Repository, h *ws.Hub) *FriendRequestAcceptedHandler {
	return &FriendRequestAcceptedHandler{usersetRepo: u, notifRepo: r, hub: h}
}

func (h *FriendRequestAcceptedHandler) EventName() event.Name { return event.FriendRequestAccepted }

func (h *FriendRequestAcceptedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("FriendRequestAccepted event received")

	var p struct {
		FriendshipID int64 `json:"friendship_id"`
		RequesterID  int64 `json:"requester_id"`
		AddresseeID  int64 `json:"addressee_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	recipient := p.RequesterID
	prefs, err := h.usersetRepo.GetNotificationPreferences(ctx, recipient)
	if err != nil {
		return fmt.Errorf("failed to load prefs: %w", err)
	}
	allow := true
	if prefs != nil && len(prefs) > 0 {
		var mp map[string]bool
		if err := json.Unmarshal(prefs, &mp); err == nil {
			if v, ok := mp[string(notification.TypeFriendRequestAccepted)]; ok {
				allow = v
			}
		}
	}
	if !allow {
		return nil
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{"friendship_id": p.FriendshipID, "addressee_id": p.AddresseeID})
	n := &notification.Notification{
		UserID:     recipient,
		ActorID:    p.AddresseeID,
		Type:       notification.TypeFriendRequestAccepted,
		EntityType: "friendship",
		EntityID:   p.FriendshipID,
		Payload:    payloadBytes,
		IsRead:     false,
		CreatedAt:  time.Now(),
	}
	if err := h.notifRepo.Create(ctx, n); err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	if err := ws.SendNotification(h.hub, n.UserID, n); err != nil {
		return err
	}
	return nil
}

type StoryPublishedHandler struct {
	friendshipRepo friendship.Repository
	storyRepo      story.StoryRepository
	usersetRepo    usersettings.Repository
	notifRepo      notification.Repository
	hub            *ws.Hub
}

func NewStoryPublishedHandler(f friendship.Repository, s story.StoryRepository, u usersettings.Repository, r notification.Repository, h *ws.Hub) *StoryPublishedHandler {
	return &StoryPublishedHandler{friendshipRepo: f, storyRepo: s, usersetRepo: u, notifRepo: r, hub: h}
}

func (h *StoryPublishedHandler) EventName() event.Name { return event.StoryPublished }

func (h *StoryPublishedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("StoryPublished event received")

	var p struct {
		StoryID    int64   `json:"story_id"`
		UserID     int64   `json:"user_id"`
		Visibility string  `json:"visibility"`
		HiddenFrom []int64 `json:"hidden_from"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	friendIDs, err := h.friendshipRepo.GetFriendIDs(ctx, p.UserID)
	if err != nil {
		return fmt.Errorf("failed to get friends: %w", err)
	}

	exclude := make(map[int64]struct{})
	for _, id := range p.HiddenFrom {
		exclude[id] = struct{}{}
	}

	var recipients []int64
	switch p.Visibility {
	case string(story.VisibilityClose):
		for _, id := range friendIDs {
			if _, ex := exclude[id]; ex {
				continue
			}
			isClose, cerr := h.storyRepo.IsCloseFriend(ctx, p.UserID, id)
			if cerr != nil {
				return fmt.Errorf("failed to check close friend: %w", cerr)
			}
			if isClose {
				recipients = append(recipients, id)
			}
		}
	default:
		for _, id := range friendIDs {
			if _, ex := exclude[id]; ex {
				continue
			}
			recipients = append(recipients, id)
		}
	}

	if len(recipients) == 0 {
		return nil
	}

	var notifs []*notification.Notification
	for _, uid := range recipients {
		prefs, err := h.usersetRepo.GetNotificationPreferences(ctx, uid)
		if err != nil {
			return fmt.Errorf("failed to load prefs: %w", err)
		}
		allow := true
		if prefs != nil && len(prefs) > 0 {
			var mp map[string]bool
			if err := json.Unmarshal(prefs, &mp); err == nil {
				if v, ok := mp[string(notification.TypeStoryPublished)]; ok {
					allow = v
				}
			}
		}
		if !allow {
			continue
		}
		payload, _ := json.Marshal(map[string]interface{}{"story_id": p.StoryID, "author_id": p.UserID})
		notifs = append(notifs, &notification.Notification{
			UserID:     uid,
			ActorID:    p.UserID,
			Type:       notification.TypeStoryPublished,
			EntityType: "story",
			EntityID:   p.StoryID,
			Payload:    payload,
			IsRead:     false,
			CreatedAt:  time.Now(),
		})
	}

	if len(notifs) == 0 {
		return nil
	}

	if err := h.notifRepo.CreateBatch(ctx, notifs); err != nil {
		return fmt.Errorf("failed to create notifications: %w", err)
	}

	for _, n := range notifs {
		if err := ws.SendNotification(h.hub, n.UserID, n); err != nil {
			return err
		}
	}

	return nil
}

type photoNotificationBase struct {
	usersetRepo usersettings.Repository
	notifRepo   notification.Repository
	hub         *ws.Hub
}

func (b photoNotificationBase) shouldDeliver(ctx context.Context, userID int64, typ notification.Type) (bool, error) {
	prefs, err := b.usersetRepo.GetNotificationPreferences(ctx, userID)
	if err != nil {
		return false, err
	}
	allow := true
	if prefs != nil && len(prefs) > 0 {
		var mp map[string]bool
		if err := json.Unmarshal(prefs, &mp); err == nil {
			if v, ok := mp[string(typ)]; ok {
				allow = v
			}
		}
	}
	return allow, nil
}

func (b photoNotificationBase) createAndPush(ctx context.Context, n *notification.Notification) error {
	if err := b.notifRepo.Create(ctx, n); err != nil {
		return err
	}
	return ws.SendNotification(b.hub, n.UserID, n)
}

type PhotoLikedHandler struct {
	photoNotificationBase
}

func NewPhotoLikedHandler(u usersettings.Repository, r notification.Repository, h *ws.Hub) *PhotoLikedHandler {
	return &PhotoLikedHandler{photoNotificationBase{usersetRepo: u, notifRepo: r, hub: h}}
}

func (h *PhotoLikedHandler) EventName() event.Name { return event.PhotoLiked }

func (h *PhotoLikedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("PhotoLiked event received")
	var p struct {
		PhotoID int64 `json:"photo_id"`
		OwnerID int64 `json:"owner_id"`
		ActorID int64 `json:"actor_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	allow, err := h.shouldDeliver(ctx, p.OwnerID, notification.TypePhotoLiked)
	if err != nil || !allow {
		return err
	}
	n := &notification.Notification{UserID: p.OwnerID, ActorID: p.ActorID, Type: notification.TypePhotoLiked, EntityType: "photo", EntityID: p.PhotoID, IsRead: false, CreatedAt: time.Now(), Payload: json.RawMessage(payload)}
	return h.createAndPush(ctx, n)
}

type PhotoCommentedHandler struct {
	photoNotificationBase
}

func NewPhotoCommentedHandler(u usersettings.Repository, r notification.Repository, h *ws.Hub) *PhotoCommentedHandler {
	return &PhotoCommentedHandler{photoNotificationBase{usersetRepo: u, notifRepo: r, hub: h}}
}

func (h *PhotoCommentedHandler) EventName() event.Name { return event.PhotoCommented }

func (h *PhotoCommentedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("PhotoCommented event received")
	var p struct {
		PhotoID   int64 `json:"photo_id"`
		CommentID int64 `json:"comment_id"`
		OwnerID   int64 `json:"owner_id"`
		ActorID   int64 `json:"actor_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	allow, err := h.shouldDeliver(ctx, p.OwnerID, notification.TypePhotoCommented)
	if err != nil || !allow {
		return err
	}
	n := &notification.Notification{UserID: p.OwnerID, ActorID: p.ActorID, Type: notification.TypePhotoCommented, EntityType: "photo", EntityID: p.PhotoID, IsRead: false, CreatedAt: time.Now(), Payload: json.RawMessage(payload)}
	return h.createAndPush(ctx, n)
}

type CommentRepliedHandler struct {
	photoNotificationBase
}

func NewCommentRepliedHandler(u usersettings.Repository, r notification.Repository, h *ws.Hub) *CommentRepliedHandler {
	return &CommentRepliedHandler{photoNotificationBase{usersetRepo: u, notifRepo: r, hub: h}}
}

func (h *CommentRepliedHandler) EventName() event.Name { return event.CommentReplied }

func (h *CommentRepliedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("CommentReplied event received")
	var p struct {
		CommentID       int64 `json:"comment_id"`
		ParentCommentID int64 `json:"parent_comment_id"`
		OwnerID         int64 `json:"owner_id"`
		ActorID         int64 `json:"actor_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	allow, err := h.shouldDeliver(ctx, p.OwnerID, notification.TypeCommentReplied)
	if err != nil || !allow {
		return err
	}
	n := &notification.Notification{UserID: p.OwnerID, ActorID: p.ActorID, Type: notification.TypeCommentReplied, EntityType: "photo_comment", EntityID: p.ParentCommentID, IsRead: false, CreatedAt: time.Now(), Payload: json.RawMessage(payload)}
	return h.createAndPush(ctx, n)
}

type CommentLikedHandler struct {
	photoNotificationBase
}

func NewCommentLikedHandler(u usersettings.Repository, r notification.Repository, h *ws.Hub) *CommentLikedHandler {
	return &CommentLikedHandler{photoNotificationBase{usersetRepo: u, notifRepo: r, hub: h}}
}

func (h *CommentLikedHandler) EventName() event.Name { return event.CommentLiked }

func (h *CommentLikedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("CommentLiked event received")
	var p struct {
		CommentID int64 `json:"comment_id"`
		OwnerID   int64 `json:"owner_id"`
		ActorID   int64 `json:"actor_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	allow, err := h.shouldDeliver(ctx, p.OwnerID, notification.TypeCommentLiked)
	if err != nil || !allow {
		return err
	}
	n := &notification.Notification{UserID: p.OwnerID, ActorID: p.ActorID, Type: notification.TypeCommentLiked, EntityType: "photo_comment", EntityID: p.CommentID, IsRead: false, CreatedAt: time.Now(), Payload: json.RawMessage(payload)}
	return h.createAndPush(ctx, n)
}

// ---------------------------------------------------------------------------
// Messenger realtime handlers
// ---------------------------------------------------------------------------

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

// — MessengerMessageSentHandler —

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
		// always push WS if connected
		if err := ws.SendMessengerEvent(h.hub, uid, "messenger.message_sent", wsPayload); err != nil {
			logger.Log.WithError(err).Warnf("messenger: failed to push to user %d", uid)
		}

		// QA-2: create push notification for offline recipients (not the sender)
		if uid == p.Message.SenderID {
			continue
		}
		if h.hub.HasUser(uid) {
			continue // already online, WS is enough
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

// — MessengerScheduledReadyHandler —

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

// — MessengerReactionHandler — (handles both added and removed)

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

// — MessengerDeliveryUpdatedHandler —

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
	// Deliver only to the sender of the original message (they need to see the tick)
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

// — MessengerMessageEditedHandler —

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

// — MessengerMessageDeletedHandler —

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

// — MessengerReadReceiptHandler —

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

// — MessengerMemberAddedHandler —

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

// — MessengerMemberRemovedHandler —

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

// — MessengerTypingHandler —

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
	// Fan-out to all members except the typer
	members, err := h.memberRepo.ListMembers(ctx, p.ConversationID)
	if err != nil {
		return fmt.Errorf("list members: %w", err)
	}
	for _, m := range members {
		if m.UserID == p.UserID {
			continue // don't echo back to sender
		}
		if err := ws.SendMessengerEvent(h.hub, m.UserID, "messenger.typing", p); err != nil {
			logger.Log.WithError(err).Warnf("typing: failed to push to user %d", m.UserID)
		}
	}
	return nil
}

// Compile-time interface checks.
var (
	_ event.Handler = (*FriendRequestReceivedHandler)(nil)
	_ event.Handler = (*FriendRequestAcceptedHandler)(nil)
	_ event.Handler = (*StoryPublishedHandler)(nil)
	_ event.Handler = (*PhotoLikedHandler)(nil)
	_ event.Handler = (*PhotoCommentedHandler)(nil)
	_ event.Handler = (*CommentRepliedHandler)(nil)
	_ event.Handler = (*CommentLikedHandler)(nil)

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
