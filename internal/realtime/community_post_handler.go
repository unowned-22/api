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

// CommunityPostHandler pushes a WS notification to all community_members
// (any role) when a community post is published — including posts created
// by the Stage 4 video-publish bridge.
type CommunityPostHandler struct {
	communityRepo community.Repository
	hub           *ws.Hub
}

func NewCommunityPostHandler(communityRepo community.Repository, hub *ws.Hub) *CommunityPostHandler {
	return &CommunityPostHandler{communityRepo: communityRepo, hub: hub}
}

func (h *CommunityPostHandler) EventName() event.Name { return event.CommunityPostPublished }

func (h *CommunityPostHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("CommunityPostPublished event received")

	var p struct {
		CommunityID int64  `json:"community_id"`
		PostID      int64  `json:"post_id"`
		Text        string `json:"text"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	members, err := h.communityRepo.ListMembers(ctx, p.CommunityID, nil, 10000, 0)
	if err != nil {
		return fmt.Errorf("listing community members: %w", err)
	}

	msg, _ := json.Marshal(map[string]any{
		"type": "community_post.published",
		"data": p,
	})

	for _, m := range members {
		h.hub.SendToUser(m.UserID, msg)
	}
	return nil
}
