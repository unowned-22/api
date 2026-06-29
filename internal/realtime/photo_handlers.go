package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/domain/usersettings"
	"github.com/unowned-22/api/internal/logger"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

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

var (
	_ event.Handler = (*PhotoLikedHandler)(nil)
	_ event.Handler = (*PhotoCommentedHandler)(nil)
	_ event.Handler = (*CommentRepliedHandler)(nil)
	_ event.Handler = (*CommentLikedHandler)(nil)
)
