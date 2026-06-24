package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/friendship"
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

func NewConsumer(cfg *config.Config, friendshipRepo friendship.Repository, storyRepo story.StoryRepository, userSettingsRepo usersettings.Repository, notificationRepo notification.Repository, hub *ws.Hub) (*Consumer, error) {
	handlers := map[event.Name]event.Handler{
		event.FriendRequestReceived: NewFriendRequestReceivedHandler(userSettingsRepo, notificationRepo, hub),
		event.FriendRequestAccepted: NewFriendRequestAcceptedHandler(userSettingsRepo, notificationRepo, hub),
		event.StoryPublished:        NewStoryPublishedHandler(friendshipRepo, storyRepo, userSettingsRepo, notificationRepo, hub),
		event.PhotoLiked:            NewPhotoLikedHandler(userSettingsRepo, notificationRepo, hub),
		event.PhotoCommented:        NewPhotoCommentedHandler(userSettingsRepo, notificationRepo, hub),
		event.CommentReplied:        NewCommentRepliedHandler(userSettingsRepo, notificationRepo, hub),
		event.CommentLiked:          NewCommentLikedHandler(userSettingsRepo, notificationRepo, hub),
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
