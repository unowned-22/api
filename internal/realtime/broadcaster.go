package realtime

import (
	"context"

	"github.com/unowned-22/api/internal/domain/notification"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

type Broadcaster struct {
	hub *ws.Hub
}

func NewBroadcaster(hub *ws.Hub) *Broadcaster {
	return &Broadcaster{hub: hub}
}

func (b *Broadcaster) Broadcast(ctx context.Context, n *notification.Notification) error {
	if n == nil || b == nil || b.hub == nil {
		return nil
	}
	return ws.SendNotification(b.hub, n.UserID, n)
}

var _ notification.Broadcaster = (*Broadcaster)(nil)
