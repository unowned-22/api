package messenger

import "time"

type ConversationType string

const (
	TypeDirect  ConversationType = "direct"
	TypeGroup   ConversationType = "group"
	TypeChannel ConversationType = "channel"
)

type Conversation struct {
	ID              int64                `json:"id"`
	Type            ConversationType     `json:"type"`
	Title           string               `json:"title"`
	Description     string               `json:"description"`
	AvatarURL       string               `json:"avatar_url"`
	OwnerID         *int64               `json:"owner_id"`
	CommunityID     *int64               `json:"community_id,omitempty"`
	CreatedBy       int64                `json:"created_by"`
	LastMessageID   *int64               `json:"last_message_id"`
	LastMessageAt   *time.Time           `json:"last_message_at"`
	MembersCount    int                  `json:"members_count"`
	IsArchived      bool                 `json:"is_archived"`
	InviteLink      string               `json:"invite_link"`
	DisappearAfterS *int                 `json:"disappear_after_s"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
	LastMessage     *Message             `json:"last_message,omitempty"`
	UnreadCount     int                  `json:"unread_count"`
	Members         []ConversationMember `json:"members,omitempty"`
	Draft           *MessageDraft        `json:"draft,omitempty"`
}
