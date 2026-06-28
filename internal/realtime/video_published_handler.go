package realtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/videosubscription"
	"github.com/unowned-22/api/internal/logger"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

type VideoPublishedHandler struct {
	subRepo videosubscription.Repository
	hub     *ws.Hub
}

func NewVideoPublishedHandler(subRepo videosubscription.Repository, hub *ws.Hub) *VideoPublishedHandler {
	return &VideoPublishedHandler{subRepo: subRepo, hub: hub}
}

func (h *VideoPublishedHandler) EventName() event.Name { return event.VideoPublished }

func (h *VideoPublishedHandler) Handle(ctx context.Context, payload []byte) error {
	logger.Log.Info("VideoPublished event received")
	var p struct {
		VideoID      int64  `json:"video_id"`
		ChannelID    int64  `json:"channel_id"`
		ChannelName  string `json:"channel_name"`
		Title        string `json:"title"`
		ThumbnailKey string `json:"thumbnail_key"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	ids, err := h.subRepo.ListSubscriberIDs(ctx, p.ChannelID)
	if err != nil {
		return err
	}
	msg, _ := json.Marshal(map[string]any{
		"type": "video.published",
		"data": p,
	})
	for _, id := range ids {
		h.hub.SendToUser(id, msg)
	}
	return nil
}
