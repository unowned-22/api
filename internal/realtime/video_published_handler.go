package realtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/unowned-22/api/internal/domain/community"
	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/logger"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

// VideoPublishedHandler notifies all subscribers of a community when a new
// video is published. After Stage 2, subscribers live in community_members
// with role=subscriber (or owner/admin who also should receive the push).
//
// Old dependency:  videosubscription.Repository.ListSubscriberIDs
// New dependency:  community.Repository.ListMembers (all roles)
type VideoPublishedHandler struct {
	communityRepo community.Repository
	hub           *ws.Hub
}

func NewVideoPublishedHandler(communityRepo community.Repository, hub *ws.Hub) *VideoPublishedHandler {
	return &VideoPublishedHandler{communityRepo: communityRepo, hub: hub}
}

func (h *VideoPublishedHandler) EventName() event.Name { return event.VideoPublished }

func (h *VideoPublishedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("VideoPublished event received")

	// JSON payload shape is unchanged for WS client compatibility:
	// channel_name is kept even though the backing entity is now a community.
	var p struct {
		VideoID      int64  `json:"video_id"`
		CommunityID  int64  `json:"community_id"`
		ChannelName  string `json:"channel_name"` // kept for WS client compat
		Title        string `json:"title"`
		ThumbnailKey string `json:"thumbnail_key"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	// Fetch all member IDs (any role) — subscribers, members, admins, and owner
	// all receive the push notification.
	members, err := h.communityRepo.ListMembers(ctx, p.CommunityID, nil, 10000, 0)
	if err != nil {
		return fmt.Errorf("listing community members: %w", err)
	}

	msg, _ := json.Marshal(map[string]any{
		"type": "video.published",
		"data": p,
	})

	for _, m := range members {
		h.hub.SendToUser(m.UserID, msg)
	}
	return nil
}
