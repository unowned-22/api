package messenger

import "time"

type MemberRole string

const (
	RoleOwner      MemberRole = "owner"
	RoleAdmin      MemberRole = "admin"
	RoleMember     MemberRole = "member"
	RoleSubscriber MemberRole = "subscriber"
)

type ConversationMember struct {
	ConversationID    int64      `json:"conversation_id"`
	UserID            int64      `json:"user_id"`
	Role              MemberRole `json:"role"`
	JoinedAt          time.Time  `json:"joined_at"`
	LeftAt            *time.Time `json:"left_at"`
	MutedUntil        *time.Time `json:"muted_until"`
	LastReadMessageID *int64     `json:"last_read_message_id"`
	LastReadAt        *time.Time `json:"last_read_at"`
	IsArchived        bool       `json:"is_archived"`
	UserName          string     `json:"user_name,omitempty"`
	UserAvatar        string     `json:"user_avatar,omitempty"`
}
