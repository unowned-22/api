package ws

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/unowned-22/api/internal/domain/messenger"
)

type MessengerMessagePayload struct {
	ConversationID int64              `json:"conversation_id"`
	Message        *messenger.Message `json:"message"`
}

type MessengerTypingPayload struct {
	ConversationID int64  `json:"conversation_id"`
	UserID         int64  `json:"user_id"`
	UserName       string `json:"user_name"`
	IsTyping       bool   `json:"is_typing"`
}

type MessengerPresencePayload struct {
	UserID     int64      `json:"user_id"`
	IsOnline   bool       `json:"is_online"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
}

type MessengerReadPayload struct {
	ConversationID    int64 `json:"conversation_id"`
	UserID            int64 `json:"user_id"`
	LastReadMessageID int64 `json:"last_read_message_id"`
}

type MessengerDeliveryPayload struct {
	ConversationID int64  `json:"conversation_id"`
	MessageID      int64  `json:"message_id"`
	UserID         int64  `json:"user_id"`
	Status         string `json:"status"`
}

type MessengerReactionPayload struct {
	ConversationID int64  `json:"conversation_id"`
	MessageID      int64  `json:"message_id"`
	UserID         int64  `json:"user_id"`
	LikesCount     int    `json:"likes_count"`
	Action         string `json:"action"`
}

type MessengerMentionPayload struct {
	ConversationID int64  `json:"conversation_id"`
	MessageID      int64  `json:"message_id"`
	SenderID       int64  `json:"sender_id"`
	SenderName     string `json:"sender_name"`
}

type MessengerPinPayload struct {
	ConversationID int64 `json:"conversation_id"`
	MessageID      int64 `json:"message_id"`
	Pinned         bool  `json:"pinned"`
	ActorID        int64 `json:"actor_id"`
}

type MessengerMessageDeletedPayload struct {
	ConversationID int64 `json:"conversation_id"`
	MessageID      int64 `json:"message_id"`
}

func SendMessengerEvent(hub *Hub, userID int64, msgType string, payload any) error {
	if hub == nil || !hub.HasUser(userID) {
		return nil
	}
	msg := WSMessage{Type: msgType, Data: payload}
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal ws message: %w", err)
	}
	hub.SendToUser(userID, b)
	return nil
}
