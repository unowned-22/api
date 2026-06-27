package dto

import (
	"encoding/json"
	"time"
)

type SendMessageRequest struct {
	Body           string   `json:"body"`
	ReplyToID      *int64   `json:"reply_to_id,omitempty"`
	AttachmentKeys []string `json:"attachment_keys,omitempty"`
}

type ScheduleMessageRequest struct {
	Body           string    `json:"body"`
	ReplyToID      *int64    `json:"reply_to_id,omitempty"`
	AttachmentKeys []string  `json:"attachment_keys,omitempty"`
	SendAt         time.Time `json:"send_at"`
}

type CreateGroupRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	MemberIDs   []int64 `json:"member_ids"`
}

type ForwardMessageRequest struct {
	TargetConversationIDs []int64 `json:"target_conversation_ids"`
}

type MarkReadRequest struct {
	LastMessageID int64 `json:"last_message_id"`
}

type UpdatePrivacyRequest struct {
	WhoCanMessage string `json:"who_can_message"`
}

type SaveDraftRequest struct {
	Body      string `json:"body"`
	ReplyToID *int64 `json:"reply_to_id,omitempty"`
}

type SetDisappearTimerRequest struct {
	DurationS int `json:"duration_s"`
}

type InviteLinkResponse struct {
	Link string `json:"link"`
}

type ConversationResponse struct {
	ID              int64      `json:"id"`
	Type            string     `json:"type"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	AvatarURL       string     `json:"avatar_url"`
	OwnerID         *int64     `json:"owner_id"`
	CreatedBy       int64      `json:"created_by"`
	LastMessageID   *int64     `json:"last_message_id"`
	LastMessageAt   *time.Time `json:"last_message_at"`
	MembersCount    int        `json:"members_count"`
	IsArchived      bool       `json:"is_archived"`
	InviteLink      string     `json:"invite_link"`
	DisappearAfterS *int       `json:"disappear_after_s"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type MessagePreviewResponse struct {
	ID         int64  `json:"id"`
	SenderID   int64  `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Body       string `json:"body"`
}

type ReactionSummaryResponse struct {
	Emoji       string `json:"emoji"`
	Count       int    `json:"count"`
	ReactedByMe bool   `json:"reacted_by_me"`
}

type AttachmentResponse struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	Filename  string `json:"filename"`
	DurationS int    `json:"duration_s,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
}

type MessengerMessageResponse struct {
	ID              int64                     `json:"id"`
	ConversationID  int64                     `json:"conversation_id"`
	SenderID        int64                     `json:"sender_id"`
	SenderName      string                    `json:"sender_name"`
	SenderAvatar    string                    `json:"sender_avatar"`
	Type            string                    `json:"type"`
	Body            string                    `json:"body"`
	ReplyToID       *int64                    `json:"reply_to_id,omitempty"`
	ReplyTo         *MessagePreviewResponse   `json:"reply_to,omitempty"`
	ForwardedFromID *int64                    `json:"forwarded_from_id,omitempty"`
	IsDeleted       bool                      `json:"is_deleted"`
	IsEdited        bool                      `json:"is_edited"`
	EditedAt        *time.Time                `json:"edited_at,omitempty"`
	Pinned          bool                      `json:"pinned"`
	Reactions       []ReactionSummaryResponse `json:"reactions"`
	DisappearsAt    *time.Time                `json:"disappears_at,omitempty"`
	ScheduledAt     *time.Time                `json:"scheduled_at,omitempty"`
	IsScheduled     bool                      `json:"is_scheduled"`
	DeliveryStatus  string                    `json:"delivery_status"`
	MentionUserIDs  []int64                   `json:"mention_user_ids"`
	Attachments     []AttachmentResponse      `json:"attachments"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

type PrivacyResponse struct {
	UserID        int64  `json:"user_id"`
	WhoCanMessage string `json:"who_can_message"`
	UpdatedAt     string `json:"updated_at"`
}

type BlockedUsersResponse struct {
	UserIDs []int64 `json:"user_ids"`
}

type ConversationsResponse struct {
	Items []ConversationResponse `json:"items"`
	Total int64                  `json:"total"`
}

type MessageListResponse struct {
	Items []MessengerMessageResponse `json:"items"`
	Total int64                      `json:"total"`
}

type UploadAttachmentResponse struct {
	StorageKey string `json:"storage_key"`
	URL        string `json:"url"`
}

var _ json.Marshaler
