package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/domain/story"
	"github.com/unowned-22/api/internal/domain/usersettings"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

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

	// build exclude set
	exclude := make(map[int64]struct{})
	for _, id := range p.HiddenFrom {
		exclude[id] = struct{}{}
	}

	var recipients []int64
	switch p.Visibility {
	case string(story.VisibilityClose):
		// include only close friends (use storyRepo.IsCloseFriend)
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
		// friends and everyone use current behavior (friends minus hidden)
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

	// For each recipient check preferences and collect notifications
	var notifs []*notification.Notification
	for _, uid := range recipients {
		prefs, err := h.usersetRepo.GetNotificationPreferences(ctx, uid)
		if err != nil {
			return fmt.Errorf("failed to load prefs: %w", err)
		}
		// default allow if missing
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

	// push to WS
	for _, n := range notifs {
		b, _ := json.Marshal(n)
		h.hub.SendToUser(n.UserID, b)
	}

	return nil
}
