package notification

import (
	"context"
	"encoding/json"
	"time"

	"github.com/unowned-22/api/internal/pagination"
)

type Type string

const (
	TypeStoryPublished        Type = "story_published"
	TypeFriendRequestReceived Type = "friend_request_received"
	TypeFriendRequestAccepted Type = "friend_request_accepted"
	TypePhotoLiked            Type = "photo_liked"
	TypePhotoCommented        Type = "photo_commented"
	TypeCommentReplied        Type = "comment_replied"
	TypeCommentLiked          Type = "comment_liked"
	TypeMessengerNewMessage   Type = "messenger_new_message"
	TypeMessengerMentioned    Type = "messenger_mentioned"
)

type Notification struct {
	ID         int64           `json:"id"`
	UserID     int64           `json:"user_id"`
	ActorID    int64           `json:"actor_id"`
	Type       Type            `json:"type"`
	EntityType string          `json:"entity_type"`
	EntityID   int64           `json:"entity_id"`
	Payload    json.RawMessage `json:"payload"`
	IsRead     bool            `json:"is_read"`
	CreatedAt  time.Time       `json:"created_at"`
}

type Repository interface {
	Create(ctx context.Context, n *Notification) error
	CreateBatch(ctx context.Context, ns []*Notification) error
	ListByUser(ctx context.Context, userID int64, page pagination.Query) ([]*Notification, int64, error)
	MarkRead(ctx context.Context, userID int64, notificationID int64) error
	MarkAllRead(ctx context.Context, userID int64) error
	CountUnread(ctx context.Context, userID int64) (int64, error)
}

// Broadcaster delivers a notification to connected realtime clients.
type Broadcaster interface {
	Broadcast(ctx context.Context, n *Notification) error
}

type Service interface {
	ListMy(ctx context.Context, userID int64, page pagination.Query) ([]*Notification, int64, error)
	UnreadCount(ctx context.Context, userID int64) (int64, error)
	MarkRead(ctx context.Context, userID int64, id int64) error
	MarkAllRead(ctx context.Context, userID int64) error
}
