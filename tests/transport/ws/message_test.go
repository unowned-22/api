package ws

import (
	"encoding/json"
	"testing"

	"github.com/unowned-22/api/internal/domain/notification"
	ws2 "github.com/unowned-22/api/internal/transport/ws"
)

func TestSendNotificationEnvelope(t *testing.T) {
	n := &notification.Notification{
		ID:   1,
		Type: notification.TypeFriendRequestReceived,
	}

	b, err := json.Marshal(ws2.WSMessage{
		Type: ws2.WSMessageNotification,
		Data: n,
	})
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got struct {
		Type string `json:"type"`
		Data struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got.Type != ws2.WSMessageNotification {
		t.Fatalf("outer type mismatch: got %q", got.Type)
	}
	if got.Data.ID != 1 {
		t.Fatalf("notification id mismatch: got %d", got.Data.ID)
	}
	if got.Data.Type != string(notification.TypeFriendRequestReceived) {
		t.Fatalf("notification type mismatch: got %q", got.Data.Type)
	}
}
