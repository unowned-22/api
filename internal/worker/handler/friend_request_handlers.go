package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/domain/usersettings"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

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
	var p struct {
		RequestID   int64 `json:"request_id"`
		RequesterID int64 `json:"requester_id"`
		AddresseeID int64 `json:"addressee_id"`
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

	payloadBytes, _ := json.Marshal(map[string]interface{}{"request_id": p.RequestID, "requester_id": p.RequesterID})
	n := &notification.Notification{
		UserID:     p.AddresseeID,
		ActorID:    p.RequesterID,
		Type:       notification.TypeFriendRequestReceived,
		EntityType: "friend_request",
		EntityID:   p.RequestID,
		Payload:    payloadBytes,
		IsRead:     false,
		CreatedAt:  time.Now(),
	}
	if err := h.notifRepo.Create(ctx, n); err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}
	b, _ := json.Marshal(n)
	h.hub.SendToUser(n.UserID, b)
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
	var p struct {
		FriendshipID int64 `json:"friendship_id"`
		RequesterID  int64 `json:"requester_id"`
		AddresseeID  int64 `json:"addressee_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	// recipient is requester
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
	b, _ := json.Marshal(n)
	h.hub.SendToUser(n.UserID, b)
	return nil
}
