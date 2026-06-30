package messenger

import (
	"context"
	"io"
	"time"

	"github.com/unowned-22/api/internal/pagination"
)

type ConversationRepository interface {
	Create(ctx context.Context, c *Conversation) (*Conversation, error)
	CreateWithMembers(ctx context.Context, c *Conversation, members []*ConversationMember) (*Conversation, error)
	GetByID(ctx context.Context, id int64) (*Conversation, error)
	GetDirect(ctx context.Context, userA, userB int64) (*Conversation, error)
	ListForUser(ctx context.Context, userID int64, page pagination.Query) ([]*Conversation, int64, error)
	Update(ctx context.Context, c *Conversation) error
	Delete(ctx context.Context, id int64) error
	UpdateLastMessage(ctx context.Context, convID, msgID int64) error
	SetInviteLink(ctx context.Context, convID int64, slug string) error
	GetByInviteLink(ctx context.Context, slug string) (*Conversation, error)
	RevokeInviteLink(ctx context.Context, convID int64) error
	SetCommunityID(ctx context.Context, conversationID int64, communityID *int64) error
	GetByCommunityID(ctx context.Context, communityID int64) (*Conversation, error)
}

type MessageRepository interface {
	CreateWithAttachments(ctx context.Context, m *Message, attachments []Attachment) (*Message, error)
	Create(ctx context.Context, m *Message) (*Message, error)
	GetByID(ctx context.Context, id int64) (*Message, error)
	List(ctx context.Context, convID int64, userID int64, page pagination.Query) ([]*Message, int64, error)
	Update(ctx context.Context, m *Message) error
	SoftDelete(ctx context.Context, id, senderID int64) error
	GetAttachments(ctx context.Context, messageID int64) ([]Attachment, error)
	CreateAttachment(ctx context.Context, a *Attachment) (*Attachment, error)
	ListPinned(ctx context.Context, convID int64, userID int64) ([]*Message, error)
	Search(ctx context.Context, convID int64, query string, page pagination.Query) ([]*Message, int64, error)
	AddReaction(ctx context.Context, messageID, userID int64, emoji string) error
	RemoveReaction(ctx context.Context, messageID, userID int64, emoji string) error
	GetReactionsSummary(ctx context.Context, messageID, viewerID int64) ([]ReactionSummary, error)
	GetReactionsSummaryBatch(ctx context.Context, messageIDs []int64, viewerID int64) (map[int64][]ReactionSummary, error)
	UpsertDeliveryStatus(ctx context.Context, s *MessageDeliveryStatus) error
	GetDeliveryStatuses(ctx context.Context, messageID int64) ([]MessageDeliveryStatus, error)
	ListExpiredDisappearing(ctx context.Context) ([]*Message, error)
	HardDeleteByID(ctx context.Context, id int64) error
	ListDueScheduled(ctx context.Context) ([]*Message, error)
	MarkScheduledSent(ctx context.Context, id int64) error
	ListMentions(ctx context.Context, userID int64, page pagination.Query) ([]*Message, int64, error)
	ListScheduled(ctx context.Context, convID, userID int64) ([]*Message, error)
	EnrichMessage(ctx context.Context, m *Message)
}

type MemberRepository interface {
	Add(ctx context.Context, m *ConversationMember) error
	Remove(ctx context.Context, convID, userID int64) error
	GetMember(ctx context.Context, convID, userID int64) (*ConversationMember, error)
	ListMembers(ctx context.Context, convID int64) ([]*ConversationMember, error)
	UpdateRole(ctx context.Context, convID, userID int64, role MemberRole) error
	MarkRead(ctx context.Context, convID, userID, lastMsgID int64) error
	UpdateMembersCount(ctx context.Context, convID int64, delta int) error
	SetArchived(ctx context.Context, convID, userID int64, archived bool) error
}

type PresenceRepository interface {
	SetOnline(ctx context.Context, userID int64) error
	SetOffline(ctx context.Context, userID int64) error
	GetPresence(ctx context.Context, userID int64) (*UserPresence, error)
	GetPresenceBatch(ctx context.Context, userIDs []int64) (map[int64]*UserPresence, error)
}

type PrivacyRepository interface {
	Get(ctx context.Context, userID int64) (*MessengerPrivacySettings, error)
	Upsert(ctx context.Context, s *MessengerPrivacySettings) error
	Block(ctx context.Context, blockerID, blockedID int64) error
	Unblock(ctx context.Context, blockerID, blockedID int64) error
	IsBlocked(ctx context.Context, blockerID, blockedID int64) (bool, error)
	ListBlocked(ctx context.Context, blockerID int64) ([]int64, error)
}

type DraftRepository interface {
	Upsert(ctx context.Context, d *MessageDraft) error
	Get(ctx context.Context, convID, userID int64) (*MessageDraft, error)
	Delete(ctx context.Context, convID, userID int64) error
	ListForUser(ctx context.Context, userID int64) ([]*MessageDraft, error)
}

type Service interface {
	GetOrCreateDirect(ctx context.Context, requesterID, targetID int64) (*Conversation, error)
	CreateGroup(ctx context.Context, creatorID int64, title, desc string, memberIDs []int64) (*Conversation, error)
	CreateChannel(ctx context.Context, ownerID int64, title, desc string) (*Conversation, error)
	GetConversation(ctx context.Context, userID, convID int64) (*Conversation, error)
	ListConversations(ctx context.Context, userID int64, page pagination.Query) ([]*Conversation, int64, error)
	ArchiveConversation(ctx context.Context, userID, convID int64) error
	UnarchiveConversation(ctx context.Context, userID, convID int64) error
	AddMembers(ctx context.Context, actorID, convID int64, memberIDs []int64) error
	RemoveMember(ctx context.Context, actorID, convID, memberID int64) error
	LeaveConversation(ctx context.Context, userID, convID int64) error
	Subscribe(ctx context.Context, userID, channelID int64) error
	GenerateInviteLink(ctx context.Context, actorID, convID int64) (string, error)
	JoinByInviteLink(ctx context.Context, userID int64, slug string) (*Conversation, error)
	RevokeInviteLink(ctx context.Context, actorID, convID int64) error
	SendMessage(ctx context.Context, senderID, convID int64, msg *Message, attachments []Attachment) (*Message, error)
	ScheduleMessage(ctx context.Context, senderID, convID int64, msg *Message, attachments []Attachment, sendAt time.Time) (*Message, error)
	EditMessage(ctx context.Context, userID, msgID int64, newBody string) (*Message, error)
	DeleteMessage(ctx context.Context, userID, msgID int64) error
	ListMessages(ctx context.Context, userID, convID int64, page pagination.Query) ([]*Message, int64, error)
	PinMessage(ctx context.Context, userID, convID, msgID int64) error
	UnpinMessage(ctx context.Context, userID, convID, msgID int64) error
	ForwardMessage(ctx context.Context, userID, msgID int64, targetConvIDs []int64) error
	ReplyToMessage(ctx context.Context, senderID, convID, replyToID int64, msg *Message, attachments []Attachment) (*Message, error)
	MarkRead(ctx context.Context, userID, convID, lastMsgID int64) error
	SearchMessages(ctx context.Context, userID, convID int64, query string, page pagination.Query) ([]*Message, int64, error)
	SetDisappearingTimer(ctx context.Context, userID, convID int64, duration time.Duration) error
	MarkDelivered(ctx context.Context, userID, msgID int64) error
	SaveDraft(ctx context.Context, userID, convID int64, body string, replyToID *int64) error
	GetDraft(ctx context.Context, userID, convID int64) (*MessageDraft, error)
	DeleteDraft(ctx context.Context, userID, convID int64) error
	CanMessage(ctx context.Context, requesterID, targetID int64) (bool, error)
	GetPrivacySettings(ctx context.Context, userID int64) (*MessengerPrivacySettings, error)
	UpdatePrivacySettings(ctx context.Context, userID int64, s *MessengerPrivacySettings) error
	BlockUser(ctx context.Context, blockerID, blockedID int64) error
	UnblockUser(ctx context.Context, blockerID, blockedID int64) error
	ListBlocked(ctx context.Context, blockerID int64) ([]int64, error)
	ListScheduledMessages(ctx context.Context, userID, convID int64) ([]*Message, error)
	CancelScheduledMessage(ctx context.Context, userID, msgID int64) error
	SendTyping(ctx context.Context, userID, convID int64, isTyping bool) error
	AddReaction(ctx context.Context, userID, msgID int64, emoji string) error
	RemoveReaction(ctx context.Context, userID, msgID int64, emoji string) error
	UploadAttachment(ctx context.Context, userID int64, filename, contentType string, body io.Reader, size int64) (storageKey, url string, err error)
	PromoteToCommunity(ctx context.Context, requesterID, conversationID int64, communityType, visibility string) (communityID int64, err error)
}
