package realtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/unowned-22/api/internal/domain/event"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

// VideoProcessingProgressHandler pushes video.processing_progress events via WebSocket
// to the video owner only. It is ephemeral — no notification or audit record is created.
type VideoProcessingProgressHandler struct {
	hub *ws.Hub
}

func NewVideoProcessingProgressHandler(hub *ws.Hub) *VideoProcessingProgressHandler {
	return &VideoProcessingProgressHandler{hub: hub}
}

func (h *VideoProcessingProgressHandler) EventName() event.Name {
	return event.VideoProcessingProgress
}

func (h *VideoProcessingProgressHandler) Handle(ctx context.Context, payload []byte) error {
	var p struct {
		VideoID    int64  `json:"video_id"`
		OwnerID    int64  `json:"owner_id"`
		Stage      string `json:"stage"`
		Percent    int    `json:"percent"`
		ETASeconds int    `json:"eta_seconds"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	// Push only to the owner — this is not a broadcast like VideoPublished.
	msg, _ := json.Marshal(map[string]any{
		"type": "video.processing_progress",
		"data": p,
	})
	h.hub.SendToUser(p.OwnerID, msg)
	return nil
}

var _ event.Handler = (*VideoProcessingProgressHandler)(nil)
