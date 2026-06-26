package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/unowned-22/api/internal/domain/messenger"
	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/pagination"
)

// ---------------------------------------------------------------------------
// Minimal mocks
// ---------------------------------------------------------------------------

// stubConvRepo satisfies messenger.ConversationRepository.
type stubConvRepo struct {
	conv             *messenger.Conversation
	updateLastMsgErr error
}

func (r *stubConvRepo) Create(ctx context.Context, c *messenger.Conversation) (*messenger.Conversation, error) {
	return c, nil
}
func (r *stubConvRepo) GetByID(ctx context.Context, id int64) (*messenger.Conversation, error) {
	return r.conv, nil
}
func (r *stubConvRepo) GetDirect(ctx context.Context, a, b int64) (*messenger.Conversation, error) {
	return nil, nil
}
func (r *stubConvRepo) ListForUser(ctx context.Context, userID int64, page pagination.Query) ([]*messenger.Conversation, int64, error) {
	return nil, 0, nil
}
func (r *stubConvRepo) Update(ctx context.Context, c *messenger.Conversation) error { return nil }
func (r *stubConvRepo) Delete(ctx context.Context, id int64) error                  { return nil }
func (r *stubConvRepo) UpdateLastMessage(ctx context.Context, convID, msgID int64) error {
	return r.updateLastMsgErr
}
func (r *stubConvRepo) SetInviteLink(ctx context.Context, convID int64, slug string) error {
	return nil
}
func (r *stubConvRepo) GetByInviteLink(ctx context.Context, slug string) (*messenger.Conversation, error) {
	return nil, nil
}
func (r *stubConvRepo) RevokeInviteLink(ctx context.Context, convID int64) error { return nil }

// stubMsgRepo satisfies messenger.MessageRepository.
type stubMsgRepo struct {
	created *messenger.Message
}

func (r *stubMsgRepo) CreateWithAttachments(ctx context.Context, m *messenger.Message, attachments []messenger.Attachment) (*messenger.Message, error) {
	m.ID = 42
	r.created = m
	return m, nil
}
func (r *stubMsgRepo) Create(ctx context.Context, m *messenger.Message) (*messenger.Message, error) {
	m.ID = 42
	return m, nil
}
func (r *stubMsgRepo) GetByID(ctx context.Context, id int64) (*messenger.Message, error) {
	return nil, errs.ErrMessageNotFound
}
func (r *stubMsgRepo) List(ctx context.Context, convID int64, page pagination.Query) ([]*messenger.Message, int64, error) {
	return nil, 0, nil
}
func (r *stubMsgRepo) Update(ctx context.Context, m *messenger.Message) error   { return nil }
func (r *stubMsgRepo) SoftDelete(ctx context.Context, id, senderID int64) error { return nil }
func (r *stubMsgRepo) GetAttachments(ctx context.Context, messageID int64) ([]messenger.Attachment, error) {
	return nil, nil
}
func (r *stubMsgRepo) CreateAttachment(ctx context.Context, a *messenger.Attachment) (*messenger.Attachment, error) {
	return a, nil
}
func (r *stubMsgRepo) ListPinned(ctx context.Context, convID int64) ([]*messenger.Message, error) {
	return nil, nil
}
func (r *stubMsgRepo) Search(ctx context.Context, convID int64, query string, page pagination.Query) ([]*messenger.Message, int64, error) {
	return nil, 0, nil
}
func (r *stubMsgRepo) AddReaction(ctx context.Context, messageID, userID int64) error { return nil }
func (r *stubMsgRepo) RemoveReaction(ctx context.Context, messageID, userID int64) error {
	return nil
}
func (r *stubMsgRepo) GetReaction(ctx context.Context, messageID, userID int64) (*messenger.Reaction, error) {
	return nil, nil
}
func (r *stubMsgRepo) UpsertDeliveryStatus(ctx context.Context, s *messenger.MessageDeliveryStatus) error {
	return nil
}
func (r *stubMsgRepo) GetDeliveryStatuses(ctx context.Context, messageID int64) ([]messenger.MessageDeliveryStatus, error) {
	return nil, nil
}
func (r *stubMsgRepo) ListExpiredDisappearing(ctx context.Context) ([]*messenger.Message, error) {
	return nil, nil
}
func (r *stubMsgRepo) HardDeleteByID(ctx context.Context, id int64) error { return nil }
func (r *stubMsgRepo) ListDueScheduled(ctx context.Context) ([]*messenger.Message, error) {
	return nil, nil
}
func (r *stubMsgRepo) MarkScheduledSent(ctx context.Context, id int64) error { return nil }
func (r *stubMsgRepo) ListMentions(ctx context.Context, userID int64, page pagination.Query) ([]*messenger.Message, int64, error) {
	return nil, 0, nil
}

// countingMemberRepo wraps a fixed member list and counts ListMembers calls.
type countingMemberRepo struct {
	members          []*messenger.ConversationMember
	listMembersCalls int
}

func (r *countingMemberRepo) Add(ctx context.Context, m *messenger.ConversationMember) error {
	return nil
}
func (r *countingMemberRepo) Remove(ctx context.Context, convID, userID int64) error { return nil }
func (r *countingMemberRepo) GetMember(ctx context.Context, convID, userID int64) (*messenger.ConversationMember, error) {
	for _, m := range r.members {
		if m.UserID == userID {
			return m, nil
		}
	}
	return nil, errs.ErrMemberNotFound
}
func (r *countingMemberRepo) ListMembers(ctx context.Context, convID int64) ([]*messenger.ConversationMember, error) {
	r.listMembersCalls++
	return r.members, nil
}
func (r *countingMemberRepo) UpdateRole(ctx context.Context, convID, userID int64, role messenger.MemberRole) error {
	return nil
}
func (r *countingMemberRepo) MarkRead(ctx context.Context, convID, userID, lastMsgID int64) error {
	return nil
}
func (r *countingMemberRepo) UpdateMembersCount(ctx context.Context, convID int64, delta int) error {
	return nil
}
func (r *countingMemberRepo) SetArchived(ctx context.Context, convID, userID int64, archived bool) error {
	return nil
}

// stubPrivacyRepo — никто никого не блокировал.
type stubPrivacyRepo struct{}

func (r *stubPrivacyRepo) Get(ctx context.Context, userID int64) (*messenger.MessengerPrivacySettings, error) {
	return nil, errs.ErrPrivacySettingsNotFound
}
func (r *stubPrivacyRepo) Upsert(ctx context.Context, s *messenger.MessengerPrivacySettings) error {
	return nil
}
func (r *stubPrivacyRepo) Block(ctx context.Context, blockerID, blockedID int64) error   { return nil }
func (r *stubPrivacyRepo) Unblock(ctx context.Context, blockerID, blockedID int64) error { return nil }
func (r *stubPrivacyRepo) IsBlocked(ctx context.Context, blockerID, blockedID int64) (bool, error) {
	return false, nil
}
func (r *stubPrivacyRepo) ListBlocked(ctx context.Context, blockerID int64) ([]int64, error) {
	return nil, nil
}

// stubDraftRepo with configurable Delete error.
type stubDraftRepo struct {
	deleteErr error
}

func (r *stubDraftRepo) Upsert(ctx context.Context, d *messenger.MessageDraft) error { return nil }
func (r *stubDraftRepo) Get(ctx context.Context, convID, userID int64) (*messenger.MessageDraft, error) {
	return nil, errs.ErrDraftNotFound
}
func (r *stubDraftRepo) Delete(ctx context.Context, convID, userID int64) error {
	return r.deleteErr
}
func (r *stubDraftRepo) ListForUser(ctx context.Context, userID int64) ([]*messenger.MessageDraft, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func directConv(id int64) *messenger.Conversation {
	return &messenger.Conversation{
		ID:   id,
		Type: messenger.TypeDirect,
	}
}

func twoMembers(convID, userA, userB int64) []*messenger.ConversationMember {
	return []*messenger.ConversationMember{
		{ConversationID: convID, UserID: userA, Role: messenger.RoleMember, JoinedAt: time.Now()},
		{ConversationID: convID, UserID: userB, Role: messenger.RoleMember, JoinedAt: time.Now()},
	}
}

func newServiceForTest(
	conv *messenger.Conversation,
	memberRepo messenger.MemberRepository,
	draftErr error,
	updateLastMsgErr error,
) *MessengerService {
	convRepo := &stubConvRepo{conv: conv, updateLastMsgErr: updateLastMsgErr}
	return NewMessengerService(
		convRepo,
		&stubMsgRepo{},
		memberRepo,
		nil,
		&stubPrivacyRepo{},
		&stubDraftRepo{deleteErr: draftErr},
		nil,
		nil,
		"",
		nil, // eventBus nil → publishMessageSentWithMembers выходит сразу
	)
}

// ---------------------------------------------------------------------------
// TASK-14.2: draftRepo.Delete failure must not fail SendMessage
// ---------------------------------------------------------------------------

func TestSendMessage_DraftDeleteError_DoesNotFail(t *testing.T) {
	const (
		convID   = int64(1)
		senderID = int64(10)
		otherID  = int64(20)
	)

	memberRepo := &countingMemberRepo{
		members: twoMembers(convID, senderID, otherID),
	}
	draftErr := errors.New("db connection lost")
	svc := newServiceForTest(directConv(convID), memberRepo, draftErr, nil)

	msg, err := svc.SendMessage(context.Background(), senderID, convID,
		&messenger.Message{Type: messenger.MessageTypeText, Body: "hi"},
		nil,
	)

	if err != nil {
		t.Fatalf("SendMessage must succeed even when draftRepo.Delete fails, got: %v", err)
	}
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
}

// ---------------------------------------------------------------------------
// TASK-14.2: UpdateLastMessage failure must not fail SendMessage
// ---------------------------------------------------------------------------

func TestSendMessage_UpdateLastMessageError_DoesNotFail(t *testing.T) {
	const (
		convID   = int64(1)
		senderID = int64(10)
		otherID  = int64(20)
	)

	memberRepo := &countingMemberRepo{
		members: twoMembers(convID, senderID, otherID),
	}
	updateErr := errors.New("deadlock detected")
	svc := newServiceForTest(directConv(convID), memberRepo, nil, updateErr)

	msg, err := svc.SendMessage(context.Background(), senderID, convID,
		&messenger.Message{Type: messenger.MessageTypeText, Body: "hi"},
		nil,
	)

	if err != nil {
		t.Fatalf("SendMessage must succeed even when UpdateLastMessage fails, got: %v", err)
	}
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
}

// ---------------------------------------------------------------------------
// TASK-14.1: ListMembers called exactly once for direct conversation
// ---------------------------------------------------------------------------

func TestSendMessage_Direct_ListMembersCalledOnce(t *testing.T) {
	const (
		convID   = int64(1)
		senderID = int64(10)
		otherID  = int64(20)
	)

	memberRepo := &countingMemberRepo{
		members: twoMembers(convID, senderID, otherID),
	}
	svc := newServiceForTest(directConv(convID), memberRepo, nil, nil)

	_, err := svc.SendMessage(context.Background(), senderID, convID,
		&messenger.Message{Type: messenger.MessageTypeText, Body: "hello"},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// eventBus == nil → publishMessageSentWithMembers не делает своего ListMembers,
	// поэтому единственный вызов — из блок-чека в SendMessage.
	if memberRepo.listMembersCalls != 1 {
		t.Errorf("expected ListMembers to be called 1 time, got %d", memberRepo.listMembersCalls)
	}
}

// ---------------------------------------------------------------------------
// TASK-13.3: block check — recipient blocked sender
// ---------------------------------------------------------------------------

func TestSendMessage_Direct_BlockedBySender_ReturnsError(t *testing.T) {
	const (
		convID   = int64(1)
		senderID = int64(10)
		otherID  = int64(20)
	)

	// privacyRepo, где otherID заблокировал senderID
	blockedPrivacy := &blockingPrivacyRepo{blockerID: otherID, blockedID: senderID}
	memberRepo := &countingMemberRepo{members: twoMembers(convID, senderID, otherID)}

	svc := NewMessengerService(
		&stubConvRepo{conv: directConv(convID)},
		&stubMsgRepo{},
		memberRepo,
		nil,
		blockedPrivacy,
		&stubDraftRepo{},
		nil, nil, "", nil,
	)

	_, err := svc.SendMessage(context.Background(), senderID, convID,
		&messenger.Message{Type: messenger.MessageTypeText, Body: "hi"},
		nil,
	)
	if !errors.Is(err, errs.ErrUserBlocked) {
		t.Errorf("expected ErrUserBlocked, got %v", err)
	}
}

// blockingPrivacyRepo возвращает true только для конкретной пары.
type blockingPrivacyRepo struct {
	blockerID int64
	blockedID int64
}

func (r *blockingPrivacyRepo) Get(ctx context.Context, userID int64) (*messenger.MessengerPrivacySettings, error) {
	return nil, errs.ErrPrivacySettingsNotFound
}
func (r *blockingPrivacyRepo) Upsert(ctx context.Context, s *messenger.MessengerPrivacySettings) error {
	return nil
}
func (r *blockingPrivacyRepo) Block(ctx context.Context, blockerID, blockedID int64) error {
	return nil
}
func (r *blockingPrivacyRepo) Unblock(ctx context.Context, blockerID, blockedID int64) error {
	return nil
}
func (r *blockingPrivacyRepo) IsBlocked(ctx context.Context, blockerID, blockedID int64) (bool, error) {
	return blockerID == r.blockerID && blockedID == r.blockedID, nil
}
func (r *blockingPrivacyRepo) ListBlocked(ctx context.Context, blockerID int64) ([]int64, error) {
	return nil, nil
}
