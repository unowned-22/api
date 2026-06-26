package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/domain/messenger"
	"github.com/unowned-22/api/internal/domain/storage"
	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/logger"
	"github.com/unowned-22/api/internal/pagination"
)

type MessengerService struct {
	convRepo     messenger.ConversationRepository
	msgRepo      messenger.MessageRepository
	memberRepo   messenger.MemberRepository
	presenceRepo messenger.PresenceRepository
	privacyRepo  messenger.PrivacyRepository
	draftRepo    messenger.DraftRepository
	friendSvc    friendship.Service
	storage      storage.Storage
	publicBucket string
	eventBus     event.Publisher
}

func NewMessengerService(
	convRepo messenger.ConversationRepository,
	msgRepo messenger.MessageRepository,
	memberRepo messenger.MemberRepository,
	presenceRepo messenger.PresenceRepository,
	privacyRepo messenger.PrivacyRepository,
	draftRepo messenger.DraftRepository,
	friendSvc friendship.Service,
	storage storage.Storage,
	publicBucket string,
	eventBus event.Publisher,
) *MessengerService {
	return &MessengerService{
		convRepo:     convRepo,
		msgRepo:      msgRepo,
		memberRepo:   memberRepo,
		presenceRepo: presenceRepo,
		privacyRepo:  privacyRepo,
		draftRepo:    draftRepo,
		friendSvc:    friendSvc,
		storage:      storage,
		publicBucket: publicBucket,
		eventBus:     eventBus,
	}
}

var mentionIDRE = regexp.MustCompile(`@([0-9]+)`)

func (s *MessengerService) CanMessage(ctx context.Context, requesterID, targetID int64) (bool, error) {
	blocked, err := s.privacyRepo.IsBlocked(ctx, targetID, requesterID)
	if err != nil {
		return false, err
	}
	if blocked {
		return false, nil
	}

	settings, err := s.privacyRepo.Get(ctx, targetID)
	if err != nil {
		if err == errs.ErrPrivacySettingsNotFound {
			return true, nil
		}
		return false, err
	}

	switch settings.WhoCanMessage {
	case messenger.WhoCanMessageEveryone:
		return true, nil
	case messenger.WhoCanMessageFriends:
		return s.friendSvc.IsFriend(ctx, requesterID, targetID)
	case messenger.WhoCanMessageNobody:
		return false, nil
	default:
		return true, nil
	}
}

func (s *MessengerService) BlockUser(ctx context.Context, blockerID, blockedID int64) error {
	if blockerID == blockedID {
		return errs.ErrCannotBlockSelf
	}
	return s.privacyRepo.Block(ctx, blockerID, blockedID)
}

func (s *MessengerService) UnblockUser(ctx context.Context, blockerID, blockedID int64) error {
	return s.privacyRepo.Unblock(ctx, blockerID, blockedID)
}

func (s *MessengerService) ListBlocked(ctx context.Context, blockerID int64) ([]int64, error) {
	return s.privacyRepo.ListBlocked(ctx, blockerID)
}

func (s *MessengerService) GetOrCreateDirect(ctx context.Context, requesterID, targetID int64) (*messenger.Conversation, error) {
	ok, err := s.CanMessage(ctx, requesterID, targetID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errs.ErrMessagingNotAllowed
	}

	if existing, err := s.convRepo.GetDirect(ctx, requesterID, targetID); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	now := time.Now().UTC()
	conv := &messenger.Conversation{
		Type:         messenger.TypeDirect,
		CreatedBy:    requesterID,
		MembersCount: 2,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	conv, err = s.convRepo.Create(ctx, conv)
	if err != nil {
		return nil, err
	}
	if err := s.memberRepo.Add(ctx, &messenger.ConversationMember{ConversationID: conv.ID, UserID: requesterID, Role: messenger.RoleMember, JoinedAt: now}); err != nil {
		return nil, err
	}
	if err := s.memberRepo.Add(ctx, &messenger.ConversationMember{ConversationID: conv.ID, UserID: targetID, Role: messenger.RoleMember, JoinedAt: now}); err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *MessengerService) CreateGroup(ctx context.Context, creatorID int64, title, desc string, memberIDs []int64) (*messenger.Conversation, error) {
	now := time.Now().UTC()
	conv := &messenger.Conversation{Type: messenger.TypeGroup, Title: title, Description: desc, CreatedBy: creatorID, OwnerID: &creatorID, MembersCount: len(memberIDs) + 1, CreatedAt: now, UpdatedAt: now}
	var err error
	conv, err = s.convRepo.Create(ctx, conv)
	if err != nil {
		return nil, err
	}
	if err := s.memberRepo.Add(ctx, &messenger.ConversationMember{ConversationID: conv.ID, UserID: creatorID, Role: messenger.RoleOwner, JoinedAt: now}); err != nil {
		return nil, err
	}
	for _, id := range memberIDs {
		if id == creatorID {
			continue
		}
		if err := s.memberRepo.Add(ctx, &messenger.ConversationMember{ConversationID: conv.ID, UserID: id, Role: messenger.RoleMember, JoinedAt: now}); err != nil {
			return nil, err
		}
	}
	return conv, nil
}

func (s *MessengerService) CreateChannel(ctx context.Context, ownerID int64, title, desc string) (*messenger.Conversation, error) {
	now := time.Now().UTC()
	conv := &messenger.Conversation{Type: messenger.TypeChannel, Title: title, Description: desc, CreatedBy: ownerID, OwnerID: &ownerID, MembersCount: 1, CreatedAt: now, UpdatedAt: now}
	var err error
	conv, err = s.convRepo.Create(ctx, conv)
	if err != nil {
		return nil, err
	}
	if err := s.memberRepo.Add(ctx, &messenger.ConversationMember{ConversationID: conv.ID, UserID: ownerID, Role: messenger.RoleOwner, JoinedAt: now}); err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *MessengerService) GetConversation(ctx context.Context, userID, convID int64) (*messenger.Conversation, error) {
	conv, err := s.convRepo.GetByID(ctx, convID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, errs.ErrConversationNotFound
	}
	if _, err := s.memberRepo.GetMember(ctx, convID, userID); err != nil {
		return nil, errs.ErrNotConversationMember
	}
	return conv, nil
}

func (s *MessengerService) ListConversations(ctx context.Context, userID int64, page pagination.Query) ([]*messenger.Conversation, int64, error) {
	return s.convRepo.ListForUser(ctx, userID, page)
}

func (s *MessengerService) ArchiveConversation(ctx context.Context, userID, convID int64) error {
	if _, err := s.memberRepo.GetMember(ctx, convID, userID); err != nil {
		return errs.ErrNotConversationMember
	}
	return s.memberRepo.SetArchived(ctx, convID, userID, true)
}

func (s *MessengerService) UnarchiveConversation(ctx context.Context, userID, convID int64) error {
	if _, err := s.memberRepo.GetMember(ctx, convID, userID); err != nil {
		return errs.ErrNotConversationMember
	}
	return s.memberRepo.SetArchived(ctx, convID, userID, false)
}

func (s *MessengerService) AddMembers(ctx context.Context, actorID, convID int64, memberIDs []int64) error {
	conv, err := s.convRepo.GetByID(ctx, convID)
	if err != nil {
		return err
	}
	if conv == nil {
		return errs.ErrConversationNotFound
	}
	if conv.Type == messenger.TypeChannel {
		member, err := s.memberRepo.GetMember(ctx, convID, actorID)
		if err != nil || member == nil {
			return errs.ErrNotConversationMember
		}
		if member.Role != messenger.RoleOwner && member.Role != messenger.RoleAdmin {
			return errs.ErrInsufficientChannelRole
		}
	}
	now := time.Now().UTC()
	added := 0
	for _, id := range memberIDs {
		if id == actorID {
			continue
		}
		if err := s.memberRepo.Add(ctx, &messenger.ConversationMember{ConversationID: convID, UserID: id, Role: messenger.RoleMember, JoinedAt: now}); err != nil {
			return err
		}
		added++
	}
	if added > 0 {
		if err := s.memberRepo.UpdateMembersCount(ctx, convID, added); err != nil {
			logger.Log.WithError(err).Warnf("MessengerService.AddMembers: failed to update members_count for conv %d", convID)
		}
	}
	// RT-5: publish member_added events
	if s.eventBus != nil {
		for _, id := range memberIDs {
			if id == actorID {
				continue
			}
			payload, _ := json.Marshal(map[string]any{
				"conversation_id": convID,
				"user_id":         id,
				"role":            "member",
				"actor_id":        actorID,
			})
			_ = s.eventBus.Publish(ctx, event.Event{Name: event.MessengerMemberAdded, Payload: payload})
		}
	}
	return nil
}

func (s *MessengerService) RemoveMember(ctx context.Context, actorID, convID, memberID int64) error {
	actor, err := s.memberRepo.GetMember(ctx, convID, actorID)
	if err != nil || actor == nil {
		return errs.ErrNotConversationMember
	}
	if actor.Role != messenger.RoleOwner && actor.Role != messenger.RoleAdmin {
		return errs.ErrInsufficientChannelRole
	}
	target, err := s.memberRepo.GetMember(ctx, convID, memberID)
	if err != nil || target == nil {
		return errs.ErrMemberNotFound
	}
	// Owner cannot be removed by anyone except themselves
	if target.Role == messenger.RoleOwner && actorID != memberID {
		return errs.ErrCannotRemoveOwner
	}
	if err := s.memberRepo.Remove(ctx, convID, memberID); err != nil {
		return err
	}
	if err := s.memberRepo.UpdateMembersCount(ctx, convID, -1); err != nil {
		logger.Log.WithError(err).Warnf("MessengerService.RemoveMember: failed to update members_count for conv %d", convID)
	}
	// RT-5: publish member_removed event
	if s.eventBus != nil {
		payload, _ := json.Marshal(map[string]any{
			"conversation_id": convID,
			"user_id":         memberID,
			"actor_id":        actorID,
		})
		_ = s.eventBus.Publish(ctx, event.Event{Name: event.MessengerMemberRemoved, Payload: payload})
	}
	return nil
}

func (s *MessengerService) LeaveConversation(ctx context.Context, userID, convID int64) error {
	if err := s.memberRepo.Remove(ctx, convID, userID); err != nil {
		return err
	}
	if err := s.memberRepo.UpdateMembersCount(ctx, convID, -1); err != nil {
		logger.Log.WithError(err).Warnf("MessengerService.LeaveConversation: failed to update members_count for conv %d", convID)
	}
	return nil
}

func (s *MessengerService) Subscribe(ctx context.Context, userID, channelID int64) error {
	if err := s.memberRepo.Add(ctx, &messenger.ConversationMember{ConversationID: channelID, UserID: userID, Role: messenger.RoleSubscriber, JoinedAt: time.Now().UTC()}); err != nil {
		return err
	}
	if err := s.memberRepo.UpdateMembersCount(ctx, channelID, 1); err != nil {
		logger.Log.WithError(err).Warnf("MessengerService.Subscribe: failed to update members_count for conv %d", channelID)
	}
	return nil
}

func randomSlug(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:n], nil
}

func (s *MessengerService) GenerateInviteLink(ctx context.Context, actorID, convID int64) (string, error) {
	conv, err := s.convRepo.GetByID(ctx, convID)
	if err != nil {
		return "", err
	}
	if conv == nil {
		return "", errs.ErrConversationNotFound
	}
	member, err := s.memberRepo.GetMember(ctx, convID, actorID)
	if err != nil || member == nil {
		return "", errs.ErrNotConversationMember
	}
	if member.Role != messenger.RoleOwner && member.Role != messenger.RoleAdmin {
		return "", errs.ErrInsufficientChannelRole
	}
	slug, err := randomSlug(12)
	if err != nil {
		return "", err
	}
	if err := s.convRepo.SetInviteLink(ctx, convID, slug); err != nil {
		return "", err
	}
	return "/join/" + slug, nil
}

func (s *MessengerService) JoinByInviteLink(ctx context.Context, userID int64, slug string) (*messenger.Conversation, error) {
	conv, err := s.convRepo.GetByInviteLink(ctx, slug)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, errs.ErrInviteLinkInvalid
	}
	role := messenger.RoleMember
	if conv.Type == messenger.TypeChannel {
		role = messenger.RoleSubscriber
	}
	if err := s.memberRepo.Add(ctx, &messenger.ConversationMember{ConversationID: conv.ID, UserID: userID, Role: role, JoinedAt: time.Now().UTC()}); err != nil {
		return nil, err
	}
	if err := s.memberRepo.UpdateMembersCount(ctx, conv.ID, 1); err != nil {
		logger.Log.WithError(err).Warnf("MessengerService.JoinByInviteLink: failed to update members_count for conv %d", conv.ID)
	}
	return conv, nil
}

func (s *MessengerService) RevokeInviteLink(ctx context.Context, actorID, convID int64) error {
	return s.convRepo.RevokeInviteLink(ctx, convID)
}

func (s *MessengerService) SendMessage(ctx context.Context, senderID, convID int64, msg *messenger.Message, attachments []messenger.Attachment) (*messenger.Message, error) {
	conv, err := s.convRepo.GetByID(ctx, convID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, errs.ErrConversationNotFound
	}
	member, err := s.memberRepo.GetMember(ctx, convID, senderID)
	if err != nil || member == nil {
		return nil, errs.ErrNotConversationMember
	}
	if conv.Type == messenger.TypeChannel && member.Role != messenger.RoleOwner && member.Role != messenger.RoleAdmin {
		return nil, errs.ErrInsufficientChannelRole
	}

	// For direct conversations we need the member list twice: once for the
	// block-check and once to embed recipient_ids in the publish payload.
	// Load it here once and pass it down to avoid a duplicate DB round-trip.
	// For group/channel convs convMembers stays nil — publishMessageSentWithMembers
	// will fetch it itself inside that path.
	var convMembers []*messenger.ConversationMember
	if conv.Type == messenger.TypeDirect {
		convMembers, err = s.memberRepo.ListMembers(ctx, convID)
		if err != nil {
			return nil, err
		}

		// TASK-13.3: check block in both directions using the already-loaded list.
		//   (a) recipient blocked sender  — sender cannot reach the recipient.
		//   (b) sender blocked recipient  — keeps block semantics symmetric.
		otherID, err := otherIDFromMembers(convMembers, senderID)
		if err != nil {
			return nil, err
		}
		blockedBySender, err := s.privacyRepo.IsBlocked(ctx, otherID, senderID)
		if err != nil {
			return nil, err
		}
		blockedByOther, err := s.privacyRepo.IsBlocked(ctx, senderID, otherID)
		if err != nil {
			return nil, err
		}
		if blockedBySender || blockedByOther {
			return nil, errs.ErrUserBlocked
		}
	}

	msg.SenderID = senderID
	msg.ConversationID = convID
	msg.MentionUserIDs = extractMentions(msg.Body)
	now := time.Now().UTC()
	msg.CreatedAt = now
	msg.UpdatedAt = now
	if conv.Type == messenger.TypeDirect {
		msg.DeliveryStatus = messenger.DeliveryStatusSent
	}

	// Stamp attachment timestamps before the atomic insert so the tx sees the
	// correct created_at values.
	for i := range attachments {
		attachments[i].CreatedAt = now
	}

	// TASK-13.2: message + attachments are written in a single transaction.
	// A failure mid-way no longer leaves an orphaned message without its
	// attachments; the entire operation is rolled back atomically.
	created, err := s.msgRepo.CreateWithAttachments(ctx, msg, attachments)
	if err != nil {
		return nil, err
	}

	// draftRepo.Delete and convRepo.UpdateLastMessage are intentionally left
	// outside the transaction: a draft that wasn't cleaned up is merely
	// cosmetic, and last_message_id will be corrected by the next successful
	// send. The critical atomicity constraint is message + attachments only.
	if err := s.draftRepo.Delete(ctx, convID, senderID); err != nil {
		logger.Log.WithError(err).Warnf("MessengerService.SendMessage: failed to delete draft for conv %d user %d", convID, senderID)
	}
	if err := s.convRepo.UpdateLastMessage(ctx, convID, created.ID); err != nil {
		logger.Log.WithError(err).Warnf("MessengerService.SendMessage: failed to update last_message_id for conv %d", convID)
	}

	if !created.IsScheduled {
		s.publishMessageSentWithMembers(ctx, convID, created, convMembers)
	}

	return created, nil
}

// otherIDFromMembers returns the UserID of the participant that is not senderID,
// operating on an already-loaded member slice (no DB call).
func otherIDFromMembers(members []*messenger.ConversationMember, senderID int64) (int64, error) {
	for _, m := range members {
		if m.UserID != senderID {
			return m.UserID, nil
		}
	}
	return 0, errs.ErrNotConversationMember
}

// messengerMessageSentPayload is the payload published for event.MessengerMessageSent.
// RecipientIDs is computed once here (active conversation members) so that the
// realtime consumer does not need to query the DB per-message.
type messengerMessageSentPayload struct {
	ConversationID int64              `json:"conversation_id"`
	Message        *messenger.Message `json:"message"`
	RecipientIDs   []int64            `json:"recipient_ids"`
}

// publishMessageSentWithMembers publishes event.MessengerMessageSent with
// recipient_ids embedded so downstream consumers can fan out without a DB hit.
//
// members may be pre-loaded by the caller (direct conversation path, where
// ListMembers was already called for the block-check). When members is nil the
// function fetches the list itself — this is the group/channel path.
func (s *MessengerService) publishMessageSentWithMembers(ctx context.Context, convID int64, msg *messenger.Message, members []*messenger.ConversationMember) {
	if s.eventBus == nil {
		return
	}
	if members == nil {
		var err error
		members, err = s.memberRepo.ListMembers(ctx, convID)
		if err != nil {
			logger.Log.WithError(err).Errorf("MessengerService: failed to list members for conv %d while publishing message_sent", convID)
			return
		}
	}
	recipientIDs := make([]int64, 0, len(members))
	for _, m := range members {
		recipientIDs = append(recipientIDs, m.UserID)
	}

	payload, err := json.Marshal(messengerMessageSentPayload{
		ConversationID: convID,
		Message:        msg,
		RecipientIDs:   recipientIDs,
	})
	if err != nil {
		logger.Log.WithError(err).Errorf("MessengerService: failed to marshal message_sent payload for conv %d", convID)
		return
	}

	if err := s.eventBus.Publish(ctx, event.Event{
		Name:    event.MessengerMessageSent,
		Payload: payload,
	}); err != nil {
		logger.Log.WithError(err).Errorf("MessengerService: failed to publish message_sent event for conv %d", convID)
	}
}

func extractMentions(body string) []int64 {
	matches := mentionIDRE.FindAllStringSubmatch(body, -1)
	out := make([]int64, 0, len(matches))
	for _, m := range matches {
		var id int64
		if _, err := fmt.Sscanf(m[1], "%d", &id); err == nil {
			out = append(out, id)
		}
	}
	return out
}

func (s *MessengerService) ScheduleMessage(ctx context.Context, senderID, convID int64, msg *messenger.Message, attachments []messenger.Attachment, sendAt time.Time) (*messenger.Message, error) {
	msg.IsScheduled = true
	msg.ScheduledAt = &sendAt
	return s.SendMessage(ctx, senderID, convID, msg, attachments)
}

func (s *MessengerService) EditMessage(ctx context.Context, userID, msgID int64, newBody string) (*messenger.Message, error) {
	msg, err := s.msgRepo.GetByID(ctx, msgID)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, errs.ErrMessageNotFound
	}
	if msg.SenderID != userID {
		return nil, errs.ErrForbidden
	}
	msg.Body = newBody
	now := time.Now().UTC()
	msg.IsEdited = true
	msg.EditedAt = &now
	if err := s.msgRepo.Update(ctx, msg); err != nil {
		return nil, err
	}
	// RT-1: broadcast edit to all conversation members
	if s.eventBus != nil {
		payload, _ := json.Marshal(map[string]any{
			"conversation_id": msg.ConversationID,
			"message_id":      msg.ID,
			"new_body":        newBody,
			"edited_at":       now,
		})
		_ = s.eventBus.Publish(ctx, event.Event{Name: event.MessengerMessageEdited, Payload: payload})
	}
	return s.msgRepo.GetByID(ctx, msgID)
}

func (s *MessengerService) DeleteMessage(ctx context.Context, userID, msgID int64) error {
	msg, err := s.msgRepo.GetByID(ctx, msgID)
	if err != nil {
		return err
	}
	if msg == nil {
		return errs.ErrMessageNotFound
	}
	if err := s.msgRepo.SoftDelete(ctx, msgID, userID); err != nil {
		return err
	}
	// RT-2: broadcast delete to all conversation members
	if s.eventBus != nil {
		payload, _ := json.Marshal(map[string]any{
			"conversation_id": msg.ConversationID,
			"message_id":      msgID,
		})
		_ = s.eventBus.Publish(ctx, event.Event{Name: event.MessengerMessageDeleted, Payload: payload})
	}
	return nil
}

func (s *MessengerService) ListMessages(ctx context.Context, userID, convID int64, page pagination.Query) ([]*messenger.Message, int64, error) {
	if _, err := s.memberRepo.GetMember(ctx, convID, userID); err != nil {
		return nil, 0, errs.ErrNotConversationMember
	}
	return s.msgRepo.List(ctx, convID, page)
}

func (s *MessengerService) PinMessage(ctx context.Context, userID, convID, msgID int64) error {
	msg, err := s.msgRepo.GetByID(ctx, msgID)
	if err != nil {
		return err
	}
	if msg == nil {
		return errs.ErrMessageNotFound
	}
	member, err := s.memberRepo.GetMember(ctx, msg.ConversationID, userID)
	if err != nil || member == nil {
		return errs.ErrNotConversationMember
	}
	if member.Role != messenger.RoleOwner && member.Role != messenger.RoleAdmin && s.convRepo != nil {
		conv, _ := s.convRepo.GetByID(ctx, msg.ConversationID)
		if conv != nil && conv.Type != messenger.TypeDirect {
			return errs.ErrInsufficientChannelRole
		}
	}
	msg.Pinned = true
	return s.msgRepo.Update(ctx, msg)
}

func (s *MessengerService) UnpinMessage(ctx context.Context, userID, convID, msgID int64) error {
	msg, err := s.msgRepo.GetByID(ctx, msgID)
	if err != nil {
		return err
	}
	if msg == nil {
		return errs.ErrMessageNotFound
	}
	if _, err := s.memberRepo.GetMember(ctx, msg.ConversationID, userID); err != nil {
		return errs.ErrNotConversationMember
	}
	msg.Pinned = false
	return s.msgRepo.Update(ctx, msg)
}

func (s *MessengerService) ForwardMessage(ctx context.Context, userID, msgID int64, targetConvIDs []int64) error {
	orig, err := s.msgRepo.GetByID(ctx, msgID)
	if err != nil {
		return err
	}
	if orig == nil {
		return errs.ErrMessageNotFound
	}
	attachments, err := s.msgRepo.GetAttachments(ctx, msgID)
	if err != nil {
		return err
	}
	for _, convID := range targetConvIDs {
		if _, err := s.memberRepo.GetMember(ctx, convID, userID); err != nil {
			return errs.ErrNotConversationMember
		}
		now := time.Now().UTC()
		copyMsg := &messenger.Message{
			ConversationID:  convID,
			SenderID:        userID,
			Type:            orig.Type,
			Body:            orig.Body,
			ForwardedFromID: &orig.ID,
			MentionUserIDs:  append([]int64(nil), orig.MentionUserIDs...),
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		copyAttachments := make([]messenger.Attachment, len(attachments))
		for i, a := range attachments {
			copyAttachments[i] = a
			copyAttachments[i].ID = 0
			copyAttachments[i].MessageID = 0
			copyAttachments[i].CreatedAt = now
		}
		if _, err := s.msgRepo.CreateWithAttachments(ctx, copyMsg, copyAttachments); err != nil {
			return err
		}
	}
	return nil
}

func (s *MessengerService) ReplyToMessage(ctx context.Context, senderID, convID, replyToID int64, msg *messenger.Message, attachments []messenger.Attachment) (*messenger.Message, error) {
	orig, err := s.msgRepo.GetByID(ctx, replyToID)
	if err != nil {
		return nil, err
	}
	if orig == nil || orig.ConversationID != convID {
		return nil, errs.ErrMessageNotFound
	}
	msg.ReplyToID = &replyToID
	return s.SendMessage(ctx, senderID, convID, msg, attachments)
}

func (s *MessengerService) MarkRead(ctx context.Context, userID, convID, lastMsgID int64) error {
	if _, err := s.memberRepo.GetMember(ctx, convID, userID); err != nil {
		return errs.ErrNotConversationMember
	}
	if err := s.memberRepo.MarkRead(ctx, convID, userID, lastMsgID); err != nil {
		return err
	}
	// RT-3: broadcast read receipt to conversation members
	if s.eventBus != nil {
		payload, _ := json.Marshal(map[string]any{
			"conversation_id":      convID,
			"user_id":              userID,
			"last_read_message_id": lastMsgID,
		})
		_ = s.eventBus.Publish(ctx, event.Event{Name: event.MessengerReadReceipt, Payload: payload})
	}
	return nil
}

func (s *MessengerService) SearchMessages(ctx context.Context, userID, convID int64, query string, page pagination.Query) ([]*messenger.Message, int64, error) {
	if _, err := s.memberRepo.GetMember(ctx, convID, userID); err != nil {
		return nil, 0, errs.ErrNotConversationMember
	}
	return s.msgRepo.Search(ctx, convID, query, page)
}

func (s *MessengerService) LikeMessage(ctx context.Context, userID, msgID int64) error {
	msg, err := s.msgRepo.GetByID(ctx, msgID)
	if err != nil {
		return err
	}
	if msg == nil {
		return errs.ErrMessageNotFound
	}
	if _, err := s.memberRepo.GetMember(ctx, msg.ConversationID, userID); err != nil {
		return errs.ErrNotConversationMember
	}
	return s.msgRepo.AddReaction(ctx, msgID, userID)
}

func (s *MessengerService) UnlikeMessage(ctx context.Context, userID, msgID int64) error {
	msg, err := s.msgRepo.GetByID(ctx, msgID)
	if err != nil {
		return err
	}
	if msg == nil {
		return errs.ErrMessageNotFound
	}
	if _, err := s.memberRepo.GetMember(ctx, msg.ConversationID, userID); err != nil {
		return errs.ErrNotConversationMember
	}
	return s.msgRepo.RemoveReaction(ctx, msgID, userID)
}

func (s *MessengerService) SetDisappearingTimer(ctx context.Context, userID, convID int64, duration time.Duration) error {
	member, err := s.memberRepo.GetMember(ctx, convID, userID)
	if err != nil || member == nil {
		return errs.ErrNotConversationMember
	}
	if member.Role != messenger.RoleOwner && member.Role != messenger.RoleAdmin {
		return errs.ErrInsufficientChannelRole
	}
	conv, err := s.convRepo.GetByID(ctx, convID)
	if err != nil {
		return err
	}
	if conv == nil {
		return errs.ErrConversationNotFound
	}
	sec := int(duration.Seconds())
	conv.DisappearAfterS = &sec
	return s.convRepo.Update(ctx, conv)
}

func (s *MessengerService) MarkDelivered(ctx context.Context, userID, msgID int64) error {
	msg, err := s.msgRepo.GetByID(ctx, msgID)
	if err != nil {
		return err
	}
	if msg == nil {
		return errs.ErrMessageNotFound
	}
	if _, err := s.memberRepo.GetMember(ctx, msg.ConversationID, userID); err != nil {
		return errs.ErrNotConversationMember
	}
	return s.msgRepo.UpsertDeliveryStatus(ctx, &messenger.MessageDeliveryStatus{MessageID: msgID, UserID: userID, Status: messenger.DeliveryStatusDelivered, UpdatedAt: time.Now().UTC()})
}

func (s *MessengerService) SaveDraft(ctx context.Context, userID, convID int64, body string, replyToID *int64) error {
	return s.draftRepo.Upsert(ctx, &messenger.MessageDraft{ConversationID: convID, UserID: userID, Body: body, ReplyToID: replyToID, UpdatedAt: time.Now().UTC()})
}

func (s *MessengerService) GetDraft(ctx context.Context, userID, convID int64) (*messenger.MessageDraft, error) {
	return s.draftRepo.Get(ctx, convID, userID)
}

func (s *MessengerService) DeleteDraft(ctx context.Context, userID, convID int64) error {
	return s.draftRepo.Delete(ctx, convID, userID)
}

func (s *MessengerService) GetPrivacySettings(ctx context.Context, userID int64) (*messenger.MessengerPrivacySettings, error) {
	settings, err := s.privacyRepo.Get(ctx, userID)
	if err != nil {
		if err == errs.ErrPrivacySettingsNotFound {
			return &messenger.MessengerPrivacySettings{
				UserID:        userID,
				WhoCanMessage: messenger.WhoCanMessageEveryone,
				UpdatedAt:     time.Now().UTC(),
			}, nil
		}
		return nil, err
	}
	return settings, nil
}

func (s *MessengerService) UpdatePrivacySettings(ctx context.Context, userID int64, settings *messenger.MessengerPrivacySettings) error {
	if settings == nil {
		settings = &messenger.MessengerPrivacySettings{}
	}
	switch settings.WhoCanMessage {
	case messenger.WhoCanMessageEveryone, messenger.WhoCanMessageFriends, messenger.WhoCanMessageNobody:
	default:
		settings.WhoCanMessage = messenger.WhoCanMessageEveryone
	}
	settings.UserID = userID
	settings.UpdatedAt = time.Now().UTC()
	return s.privacyRepo.Upsert(ctx, settings)
}

func (s *MessengerService) ListScheduledMessages(ctx context.Context, userID, convID int64) ([]*messenger.Message, error) {
	if _, err := s.memberRepo.GetMember(ctx, convID, userID); err != nil {
		return nil, errs.ErrNotConversationMember
	}
	return s.msgRepo.ListScheduled(ctx, convID, userID)
}

func (s *MessengerService) CancelScheduledMessage(ctx context.Context, userID, msgID int64) error {
	msg, err := s.msgRepo.GetByID(ctx, msgID)
	if err != nil {
		return err
	}
	if msg == nil {
		return errs.ErrMessageNotFound
	}
	if msg.SenderID != userID {
		return errs.ErrForbidden
	}
	if !msg.IsScheduled {
		return errs.ErrMessageNotScheduled
	}
	return s.msgRepo.HardDeleteByID(ctx, msgID)
}

var _ messenger.Service = (*MessengerService)(nil)
