package messenger

import "time"

type MessageType string

const (
	MessageTypeText   MessageType = "text"
	MessageTypeImage  MessageType = "image"
	MessageTypeFile   MessageType = "file"
	MessageTypeAudio  MessageType = "audio"
	MessageTypeVideo  MessageType = "video"
	MessageTypeSystem MessageType = "system"
)

type DeliveryStatus string

const (
	DeliveryStatusSent      DeliveryStatus = "sent"
	DeliveryStatusDelivered DeliveryStatus = "delivered"
	DeliveryStatusRead      DeliveryStatus = "read"
)

type Message struct {
	ID              int64          `json:"id"`
	ConversationID  int64          `json:"conversation_id"`
	SenderID        int64          `json:"sender_id"`
	Type            MessageType    `json:"type"`
	Body            string         `json:"body"`
	ReplyToID       *int64         `json:"reply_to_id"`
	ForwardedFromID *int64         `json:"forwarded_from_id"`
	IsDeleted       bool           `json:"is_deleted"`
	IsEdited        bool           `json:"is_edited"`
	EditedAt        *time.Time     `json:"edited_at"`
	Pinned          bool           `json:"pinned"`
	LikesCount      int            `json:"likes_count"`
	DisappearsAt    *time.Time     `json:"disappears_at"`
	ScheduledAt     *time.Time     `json:"scheduled_at"`
	IsScheduled     bool           `json:"is_scheduled"`
	DeliveryStatus  DeliveryStatus `json:"delivery_status"`
	MentionUserIDs  []int64        `json:"mention_user_ids"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	Attachments     []Attachment   `json:"attachments,omitempty"`
	SenderName      string         `json:"sender_name,omitempty"`
	SenderAvatar    string         `json:"sender_avatar,omitempty"`
	ReplyTo         *Message       `json:"reply_to,omitempty"`
	LikedByMe       bool           `json:"liked_by_me"`
}

type Attachment struct {
	ID           int64     `json:"id"`
	MessageID    int64     `json:"message_id"`
	Type         string    `json:"type"`
	StorageKey   string    `json:"storage_key"`
	URL          string    `json:"url"`
	MimeType     string    `json:"mime_type"`
	SizeBytes    int64     `json:"size_bytes"`
	Filename     string    `json:"filename"`
	DurationS    int       `json:"duration_s"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	ThumbnailKey string    `json:"thumbnail_key"`
	CreatedAt    time.Time `json:"created_at"`
}

type Reaction struct {
	MessageID int64     `json:"message_id"`
	UserID    int64     `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type MessageDeliveryStatus struct {
	MessageID int64          `json:"message_id"`
	UserID    int64          `json:"user_id"`
	Status    DeliveryStatus `json:"status"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type MessageDraft struct {
	ConversationID int64     `json:"conversation_id"`
	UserID         int64     `json:"user_id"`
	Body           string    `json:"body"`
	ReplyToID      *int64    `json:"reply_to_id"`
	UpdatedAt      time.Time `json:"updated_at"`
}
