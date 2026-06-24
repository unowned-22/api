package ws

import (
	"encoding/json"
	"fmt"

	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/logger"
)

const (
	WSMessageNotification = "notification"
	WSMessagePing         = "ping"
)

type WSMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

func SendNotification(hub *Hub, userID int64, n *notification.Notification) error {
	if hub == nil || n == nil {
		return nil
	}

	if !hub.HasUser(userID) {
		logger.Log.WithField("user_id", userID).Info("User not connected")
		return nil
	}

	msg := WSMessage{
		Type: WSMessageNotification,
		Data: n,
	}

	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal websocket notification: %w", err)
	}

	hub.SendToUser(userID, b)
	logger.Log.WithFields(map[string]interface{}{
		"user_id":           userID,
		"notification_id":   n.ID,
		"notification_type": n.Type,
	}).Info("WebSocket notification sent")

	return nil
}
