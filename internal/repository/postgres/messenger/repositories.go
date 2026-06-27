package messenger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	domainmessenger "github.com/unowned-22/api/internal/domain/messenger"
	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/pagination"
)

type ConversationRepository struct{ db *pgxpool.Pool }
type MessageRepository struct{ db *pgxpool.Pool }
type MemberRepository struct{ db *pgxpool.Pool }
type PresenceRepository struct{ db *pgxpool.Pool }
type PrivacyRepository struct{ db *pgxpool.Pool }
type DraftRepository struct{ db *pgxpool.Pool }

func NewConversationRepository(db *pgxpool.Pool) *ConversationRepository {
	return &ConversationRepository{db: db}
}
func NewMessageRepository(db *pgxpool.Pool) *MessageRepository   { return &MessageRepository{db: db} }
func NewMemberRepository(db *pgxpool.Pool) *MemberRepository     { return &MemberRepository{db: db} }
func NewPresenceRepository(db *pgxpool.Pool) *PresenceRepository { return &PresenceRepository{db: db} }
func NewPrivacyRepository(db *pgxpool.Pool) *PrivacyRepository   { return &PrivacyRepository{db: db} }
func NewDraftRepository(db *pgxpool.Pool) *DraftRepository       { return &DraftRepository{db: db} }

func scanConversation(row pgx.Row) (*domainmessenger.Conversation, error) {
	var c domainmessenger.Conversation
	var ownerID *int64
	var lastMessageID *int64
	var lastMessageAt *time.Time
	err := row.Scan(&c.ID, &c.Type, &c.Title, &c.Description, &c.AvatarURL, &ownerID, &c.CreatedBy, &lastMessageID, &lastMessageAt, &c.MembersCount, &c.IsArchived, &c.InviteLink, &c.DisappearAfterS, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.OwnerID = ownerID
	c.LastMessageID = lastMessageID
	c.LastMessageAt = lastMessageAt
	return &c, nil
}

func scanMessage(row pgx.Row) (*domainmessenger.Message, error) {
	var m domainmessenger.Message
	var replyToID *int64
	var forwardedFromID *int64
	var editedAt *time.Time
	var disappearsAt *time.Time
	var scheduledAt *time.Time
	var mentions []int64
	err := row.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Type, &m.Body, &replyToID, &forwardedFromID, &m.IsDeleted, &m.IsEdited, &editedAt, &m.Pinned, &m.LikesCount, &disappearsAt, &scheduledAt, &m.IsScheduled, &m.DeliveryStatus, &mentions, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	m.ReplyToID = replyToID
	m.ForwardedFromID = forwardedFromID
	m.EditedAt = editedAt
	m.DisappearsAt = disappearsAt
	m.ScheduledAt = scheduledAt
	m.MentionUserIDs = mentions
	return &m, nil
}

func (r *MessageRepository) GetByIDWithAttachments(ctx context.Context, id int64) (*domainmessenger.Message, error) {
	m, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	atts, err := r.GetAttachments(ctx, id)
	if err != nil {
		return nil, err
	}
	m.Attachments = atts
	return m, nil
}

func scanAttachment(row pgx.Row) (*domainmessenger.Attachment, error) {
	var a domainmessenger.Attachment
	err := row.Scan(&a.ID, &a.MessageID, &a.Type, &a.StorageKey, &a.URL, &a.MimeType, &a.SizeBytes, &a.Filename, &a.DurationS, &a.Width, &a.Height, &a.ThumbnailKey, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func scanMember(row pgx.Row) (*domainmessenger.ConversationMember, error) {
	var m domainmessenger.ConversationMember
	var leftAt *time.Time
	var mutedUntil *time.Time
	var lastReadID *int64
	var lastReadAt *time.Time
	err := row.Scan(&m.ConversationID, &m.UserID, &m.Role, &m.JoinedAt, &leftAt, &mutedUntil, &lastReadID, &lastReadAt, &m.IsArchived)
	if err != nil {
		return nil, err
	}
	m.LeftAt = leftAt
	m.MutedUntil = mutedUntil
	m.LastReadMessageID = lastReadID
	m.LastReadAt = lastReadAt
	return &m, nil
}

func mapPgErr(err error, notFound error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return notFound
		case "23503":
			return notFound
		}
	}
	return err
}

func (r *ConversationRepository) Create(ctx context.Context, c *domainmessenger.Conversation) (*domainmessenger.Conversation, error) {
	q := `INSERT INTO conversations (type, title, description, avatar_url, owner_id, created_by, last_message_id, last_message_at, members_count, is_archived, invite_link, disappear_after_s, created_at, updated_at)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id`
	if err := r.db.QueryRow(ctx, q, c.Type, c.Title, c.Description, c.AvatarURL, c.OwnerID, c.CreatedBy, c.LastMessageID, c.LastMessageAt, c.MembersCount, c.IsArchived, c.InviteLink, c.DisappearAfterS, c.CreatedAt, c.UpdatedAt).Scan(&c.ID); err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	return c, nil
}

func (r *ConversationRepository) CreateWithMembers(ctx context.Context, c *domainmessenger.Conversation, members []*domainmessenger.ConversationMember) (*domainmessenger.Conversation, error) {
	convQ := `INSERT INTO conversations (type, title, description, avatar_url, owner_id, created_by, last_message_id, last_message_at, members_count, is_archived, invite_link, disappear_after_s, created_at, updated_at)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id`
	memberQ := `INSERT INTO conversation_members (conversation_id,user_id,role,joined_at,left_at,muted_until,last_read_message_id,last_read_at,is_archived) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, convQ, c.Type, c.Title, c.Description, c.AvatarURL, c.OwnerID, c.CreatedBy, c.LastMessageID, c.LastMessageAt, c.MembersCount, c.IsArchived, c.InviteLink, c.DisappearAfterS, c.CreatedAt, c.UpdatedAt).Scan(&c.ID); err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	for _, m := range members {
		m.ConversationID = c.ID
		if _, err := tx.Exec(ctx, memberQ, m.ConversationID, m.UserID, m.Role, m.JoinedAt, m.LeftAt, m.MutedUntil, m.LastReadMessageID, m.LastReadAt, m.IsArchived); err != nil {
			return nil, fmt.Errorf("add member %d: %w", m.UserID, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return c, nil
}

func (r *ConversationRepository) GetByID(ctx context.Context, id int64) (*domainmessenger.Conversation, error) {
	q := `SELECT id, type, title, description, avatar_url, owner_id, created_by, last_message_id, last_message_at, members_count, is_archived, invite_link, disappear_after_s, created_at, updated_at FROM conversations WHERE id = $1`
	c, err := scanConversation(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrConversationNotFound
		}
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	return c, nil
}

func (r *ConversationRepository) GetDirect(ctx context.Context, userA, userB int64) (*domainmessenger.Conversation, error) {
	q := `SELECT c.id, c.type, c.title, c.description, c.avatar_url, c.owner_id, c.created_by, c.last_message_id, c.last_message_at, c.members_count, c.is_archived, c.invite_link, c.disappear_after_s, c.created_at, c.updated_at
	FROM conversations c
	JOIN conversation_members m1 ON m1.conversation_id = c.id AND m1.user_id = $1 AND m1.left_at IS NULL
	JOIN conversation_members m2 ON m2.conversation_id = c.id AND m2.user_id = $2 AND m2.left_at IS NULL
	WHERE c.type = 'direct' LIMIT 1`
	c, err := scanConversation(r.db.QueryRow(ctx, q, userA, userB))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get direct conversation: %w", err)
	}
	return c, nil
}

func (r *ConversationRepository) ListForUser(ctx context.Context, userID int64, page pagination.Query) ([]*domainmessenger.Conversation, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM conversations c JOIN conversation_members m ON m.conversation_id = c.id AND m.user_id = $1 AND m.left_at IS NULL`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count conversations: %w", err)
	}

	q := `
SELECT
    c.id, c.type, c.title, c.description, c.avatar_url, c.owner_id, c.created_by,
    c.last_message_id, c.last_message_at, c.members_count, c.is_archived, c.invite_link,
    c.disappear_after_s, c.created_at, c.updated_at,
    (
        SELECT COUNT(*)
        FROM messages m2
        WHERE m2.conversation_id = c.id
          AND m2.id > COALESCE(cm.last_read_message_id, 0)
          AND m2.sender_id != $1
          AND m2.is_deleted = FALSE
          AND m2.is_scheduled = FALSE
    ) AS unread_count,
    lm.id, lm.conversation_id, lm.sender_id, lm.type, lm.body,
    lm.reply_to_id, lm.forwarded_from_id, lm.is_deleted, lm.is_edited,
    lm.edited_at, lm.pinned, lm.likes_count, lm.disappears_at, lm.scheduled_at,
    lm.is_scheduled, lm.delivery_status, lm.mention_user_ids, lm.created_at, lm.updated_at
FROM conversations c
JOIN conversation_members cm ON cm.conversation_id = c.id AND cm.user_id = $1 AND cm.left_at IS NULL
LEFT JOIN messages lm ON lm.id = c.last_message_id
ORDER BY COALESCE(c.last_message_at, c.created_at) DESC
LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, q, userID, page.Limit, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	out := make([]*domainmessenger.Conversation, 0)
	for rows.Next() {
		var c domainmessenger.Conversation
		var ownerID *int64
		var lastMessageID *int64
		var lastMessageAt *time.Time
		var unreadCount int

		// last message fields (all nullable due to LEFT JOIN)
		var lmID *int64
		var lmConvID, lmSenderID *int64
		var lmType *string
		var lmBody *string
		var lmReplyToID, lmForwardedFromID *int64
		var lmIsDeleted, lmIsEdited, lmPinned, lmIsScheduled *bool
		var lmEditedAt, lmDisappearsAt, lmScheduledAt *time.Time
		var lmLikesCount *int
		var lmDeliveryStatus *string
		var lmMentions []int64
		var lmCreatedAt, lmUpdatedAt *time.Time

		if err := rows.Scan(
			&c.ID, &c.Type, &c.Title, &c.Description, &c.AvatarURL, &ownerID, &c.CreatedBy,
			&lastMessageID, &lastMessageAt, &c.MembersCount, &c.IsArchived, &c.InviteLink,
			&c.DisappearAfterS, &c.CreatedAt, &c.UpdatedAt,
			&unreadCount,
			&lmID, &lmConvID, &lmSenderID, &lmType, &lmBody,
			&lmReplyToID, &lmForwardedFromID, &lmIsDeleted, &lmIsEdited,
			&lmEditedAt, &lmPinned, &lmLikesCount, &lmDisappearsAt, &lmScheduledAt,
			&lmIsScheduled, &lmDeliveryStatus, &lmMentions, &lmCreatedAt, &lmUpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan conversation: %w", err)
		}
		c.OwnerID = ownerID
		c.LastMessageID = lastMessageID
		c.LastMessageAt = lastMessageAt
		c.UnreadCount = unreadCount

		if lmID != nil {
			msg := &domainmessenger.Message{
				ID:              *lmID,
				ConversationID:  *lmConvID,
				SenderID:        *lmSenderID,
				Type:            domainmessenger.MessageType(*lmType),
				Body:            *lmBody,
				ReplyToID:       lmReplyToID,
				ForwardedFromID: lmForwardedFromID,
				IsDeleted:       derefBool(lmIsDeleted),
				IsEdited:        derefBool(lmIsEdited),
				EditedAt:        lmEditedAt,
				Pinned:          derefBool(lmPinned),
				LikesCount:      derefInt(lmLikesCount),
				DisappearsAt:    lmDisappearsAt,
				ScheduledAt:     lmScheduledAt,
				IsScheduled:     derefBool(lmIsScheduled),
				MentionUserIDs:  lmMentions,
			}
			if lmDeliveryStatus != nil {
				msg.DeliveryStatus = domainmessenger.DeliveryStatus(*lmDeliveryStatus)
			}
			if lmCreatedAt != nil {
				msg.CreatedAt = *lmCreatedAt
			}
			if lmUpdatedAt != nil {
				msg.UpdatedAt = *lmUpdatedAt
			}
			c.LastMessage = msg
		}
		out = append(out, &c)
	}
	return out, total, nil
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func (r *ConversationRepository) Update(ctx context.Context, c *domainmessenger.Conversation) error {
	_, err := r.db.Exec(ctx, `UPDATE conversations SET type=$1,title=$2,description=$3,avatar_url=$4,owner_id=$5,created_by=$6,last_message_id=$7,last_message_at=$8,members_count=$9,is_archived=$10,invite_link=$11,disappear_after_s=$12,updated_at=NOW() WHERE id=$13`,
		c.Type, c.Title, c.Description, c.AvatarURL, c.OwnerID, c.CreatedBy, c.LastMessageID, c.LastMessageAt, c.MembersCount, c.IsArchived, c.InviteLink, c.DisappearAfterS, c.ID)
	return err
}

func (r *ConversationRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM conversations WHERE id = $1`, id)
	return err
}

func (r *ConversationRepository) UpdateLastMessage(ctx context.Context, convID, msgID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE conversations SET last_message_id=$2, last_message_at=NOW(), updated_at=NOW() WHERE id=$1`, convID, msgID)
	return err
}

func (r *ConversationRepository) SetInviteLink(ctx context.Context, convID int64, slug string) error {
	_, err := r.db.Exec(ctx, `UPDATE conversations SET invite_link=$2, updated_at=NOW() WHERE id=$1`, convID, slug)
	return err
}
func (r *ConversationRepository) GetByInviteLink(ctx context.Context, slug string) (*domainmessenger.Conversation, error) {
	c, err := scanConversation(r.db.QueryRow(ctx, `SELECT id, type, title, description, avatar_url, owner_id, created_by, last_message_id, last_message_at, members_count, is_archived, invite_link, disappear_after_s, created_at, updated_at FROM conversations WHERE invite_link=$1`, slug))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}
func (r *ConversationRepository) RevokeInviteLink(ctx context.Context, convID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE conversations SET invite_link=NULL, updated_at=NOW() WHERE id=$1`, convID)
	return err
}

// CreateWithAttachments inserts a message and all its attachments atomically.
// If any attachment insert fails the entire transaction is rolled back, so
// the caller never sees a message with a partial set of attachments.
func (r *MessageRepository) CreateWithAttachments(ctx context.Context, m *domainmessenger.Message, attachments []domainmessenger.Attachment) (*domainmessenger.Message, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := `INSERT INTO messages (conversation_id, sender_id, type, body, reply_to_id, forwarded_from_id, is_deleted, is_edited, edited_at, pinned, likes_count, disappears_at, scheduled_at, is_scheduled, delivery_status, mention_user_ids, created_at, updated_at)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) RETURNING id`
	if err := tx.QueryRow(ctx, q, m.ConversationID, m.SenderID, m.Type, m.Body, m.ReplyToID, m.ForwardedFromID, m.IsDeleted, m.IsEdited, m.EditedAt, m.Pinned, m.LikesCount, m.DisappearsAt, m.ScheduledAt, m.IsScheduled, m.DeliveryStatus, m.MentionUserIDs, m.CreatedAt, m.UpdatedAt).Scan(&m.ID); err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}

	for i := range attachments {
		a := &attachments[i]
		a.MessageID = m.ID
		if err := tx.QueryRow(ctx,
			`INSERT INTO message_attachments (message_id, type, storage_key, url, mime_type, size_bytes, filename, duration_s, width, height, thumbnail_key, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
			a.MessageID, a.Type, a.StorageKey, a.URL, a.MimeType, a.SizeBytes, a.Filename, a.DurationS, a.Width, a.Height, a.ThumbnailKey, a.CreatedAt,
		).Scan(&a.ID); err != nil {
			return nil, fmt.Errorf("insert attachment: %w", err)
		}
		m.Attachments = append(m.Attachments, *a)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return m, nil
}

func (r *MessageRepository) Create(ctx context.Context, m *domainmessenger.Message) (*domainmessenger.Message, error) {
	q := `INSERT INTO messages (conversation_id, sender_id, type, body, reply_to_id, forwarded_from_id, is_deleted, is_edited, edited_at, pinned, likes_count, disappears_at, scheduled_at, is_scheduled, delivery_status, mention_user_ids, created_at, updated_at)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) RETURNING id`
	if err := r.db.QueryRow(ctx, q, m.ConversationID, m.SenderID, m.Type, m.Body, m.ReplyToID, m.ForwardedFromID, m.IsDeleted, m.IsEdited, m.EditedAt, m.Pinned, m.LikesCount, m.DisappearsAt, m.ScheduledAt, m.IsScheduled, m.DeliveryStatus, m.MentionUserIDs, m.CreatedAt, m.UpdatedAt).Scan(&m.ID); err != nil {
		return nil, err
	}
	return m, nil
}
func (r *MessageRepository) GetByID(ctx context.Context, id int64) (*domainmessenger.Message, error) {
	m, err := scanMessage(r.db.QueryRow(ctx, `SELECT id, conversation_id, sender_id, type, body, reply_to_id, forwarded_from_id, is_deleted, is_edited, edited_at, pinned, likes_count, disappears_at, scheduled_at, is_scheduled, delivery_status, mention_user_ids, created_at, updated_at FROM messages WHERE id=$1`, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrMessageNotFound
		}
		return nil, err
	}
	return m, nil
}
func (r *MessageRepository) List(ctx context.Context, convID int64, userID int64, page pagination.Query) ([]*domainmessenger.Message, int64, error) {
	return r.listMessagesByQuery(ctx, userID, `WHERE conversation_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, convID, page.Limit, page.Offset())
}

func (r *MessageRepository) listMessagesByQuery(ctx context.Context, userID int64, where string, args ...any) ([]*domainmessenger.Message, int64, error) {
	var total int64
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE conversation_id=$1`, args[0]).Scan(&total)

	q := `SELECT id, conversation_id, sender_id, type, body, reply_to_id, forwarded_from_id,
                 is_deleted, is_edited, edited_at, pinned, likes_count, disappears_at,
                 scheduled_at, is_scheduled, delivery_status, mention_user_ids, created_at, updated_at
          FROM messages ` + where
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]*domainmessenger.Message, 0)
	ids := make([]int64, 0)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, m)
		ids = append(ids, m.ID)
	}

	if len(ids) > 0 {
		attachMap, err := r.GetAttachmentsBatch(ctx, ids)
		if err != nil {
			return nil, 0, err
		}
		reactionsMap, err := r.GetReactionsSummaryBatch(ctx, ids, userID)
		if err != nil {
			return nil, 0, err
		}

		senderIDs := make([]int64, 0, len(out))
		for _, m := range out {
			senderIDs = append(senderIDs, m.SenderID)
		}
		senders, err := r.GetSendersBatch(ctx, senderIDs)
		if err != nil {
			return nil, 0, err
		}

		for _, m := range out {
			if aa, ok := attachMap[m.ID]; ok {
				m.Attachments = aa
			}
			m.Reactions = reactionsMap[m.ID]
			if s, ok := senders[m.SenderID]; ok {
				m.SenderName = s.Name
				m.SenderAvatar = s.Avatar
			}
		}
	}

	return out, total, nil
}

func (r *MessageRepository) GetSendersBatch(ctx context.Context, senderIDs []int64) (map[int64]struct{ Name, Avatar string }, error) {
	out := make(map[int64]struct{ Name, Avatar string })
	if len(senderIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, COALESCE(full_name, username, ''), COALESCE(avatar_url, '') FROM users WHERE id = ANY($1)`,
		senderIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var v struct{ Name, Avatar string }
		if err := rows.Scan(&id, &v.Name, &v.Avatar); err != nil {
			return nil, err
		}
		out[id] = v
	}
	return out, nil
}

func (r *MessageRepository) EnrichMessage(ctx context.Context, m *domainmessenger.Message) {
	_ = r.db.QueryRow(ctx,
		`SELECT COALESCE(full_name, username, ''), COALESCE(avatar_url, '')
         FROM users WHERE id = $1`,
		m.SenderID,
	).Scan(&m.SenderName, &m.SenderAvatar)

	if m.ReplyToID != nil && m.ReplyTo == nil {
		orig, err := r.GetByID(ctx, *m.ReplyToID)
		if err == nil && orig != nil {
			_ = r.db.QueryRow(ctx,
				`SELECT COALESCE(full_name, username, '') FROM users WHERE id = $1`,
				orig.SenderID,
			).Scan(&orig.SenderName)
			m.ReplyTo = orig
		}
	}
}

func (r *MessageRepository) Update(ctx context.Context, m *domainmessenger.Message) error {
	_, err := r.db.Exec(ctx, `UPDATE messages SET body=$1,is_edited=$2,edited_at=$3,pinned=$4,likes_count=$5,disappears_at=$6,scheduled_at=$7,is_scheduled=$8,delivery_status=$9,mention_user_ids=$10,updated_at=NOW() WHERE id=$11`, m.Body, m.IsEdited, m.EditedAt, m.Pinned, m.LikesCount, m.DisappearsAt, m.ScheduledAt, m.IsScheduled, m.DeliveryStatus, m.MentionUserIDs, m.ID)
	return err
}
func (r *MessageRepository) SoftDelete(ctx context.Context, id, senderID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE messages SET is_deleted=TRUE, body='[deleted]', updated_at=NOW() WHERE id=$1 AND sender_id=$2`, id, senderID)
	return err
}
func (r *MessageRepository) GetAttachments(ctx context.Context, messageID int64) ([]domainmessenger.Attachment, error) {
	rows, err := r.db.Query(ctx, `SELECT id, message_id, type, storage_key, url, mime_type, size_bytes, filename, duration_s, width, height, thumbnail_key, created_at FROM message_attachments WHERE message_id=$1 ORDER BY id`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domainmessenger.Attachment{}
	for rows.Next() {
		var a domainmessenger.Attachment
		if err := rows.Scan(&a.ID, &a.MessageID, &a.Type, &a.StorageKey, &a.URL, &a.MimeType, &a.SizeBytes, &a.Filename, &a.DurationS, &a.Width, &a.Height, &a.ThumbnailKey, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}
func (r *MessageRepository) CreateAttachment(ctx context.Context, a *domainmessenger.Attachment) (*domainmessenger.Attachment, error) {
	if err := r.db.QueryRow(ctx, `INSERT INTO message_attachments (message_id, type, storage_key, url, mime_type, size_bytes, filename, duration_s, width, height, thumbnail_key, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`, a.MessageID, a.Type, a.StorageKey, a.URL, a.MimeType, a.SizeBytes, a.Filename, a.DurationS, a.Width, a.Height, a.ThumbnailKey, a.CreatedAt).Scan(&a.ID); err != nil {
		return nil, err
	}
	return a, nil
}
func (r *MessageRepository) ListPinned(ctx context.Context, convID int64, userID int64) ([]*domainmessenger.Message, error) {
	msgs, _, err := r.listMessagesByQuery(ctx, userID, `WHERE conversation_id=$1 AND pinned=TRUE ORDER BY created_at DESC`, convID)
	return msgs, err
}
func (r *MessageRepository) Search(ctx context.Context, convID int64, query string, page pagination.Query) ([]*domainmessenger.Message, int64, error) {
	var total int64
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE conversation_id=$1 AND is_deleted=FALSE AND (to_tsvector('russian', COALESCE(body,'')) @@ plainto_tsquery('russian', $2) OR to_tsvector('simple', COALESCE(body,'')) @@ plainto_tsquery('simple', $2))`, convID, query).Scan(&total)
	rows, err := r.db.Query(ctx, `SELECT id, conversation_id, sender_id, type, body, reply_to_id, forwarded_from_id, is_deleted, is_edited, edited_at, pinned, likes_count, disappears_at, scheduled_at, is_scheduled, delivery_status, mention_user_ids, created_at, updated_at FROM messages WHERE conversation_id=$1 AND is_deleted=FALSE AND (to_tsvector('russian', COALESCE(body,'')) @@ plainto_tsquery('russian', $2) OR to_tsvector('simple', COALESCE(body,'')) @@ plainto_tsquery('simple', $2)) ORDER BY created_at DESC LIMIT $3 OFFSET $4`, convID, query, page.Limit, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*domainmessenger.Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	return out, total, nil
}

func (r *MessageRepository) AddReaction(ctx context.Context, messageID, userID int64, emoji string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cmd, err := tx.Exec(ctx,
		`INSERT INTO message_reactions (message_id, user_id, emoji) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
		messageID, userID, emoji)
	if err != nil {
		return fmt.Errorf("insert message reaction: %w", err)
	}
	if cmd.RowsAffected() > 0 {
		if _, err := tx.Exec(ctx, `UPDATE messages SET likes_count = likes_count + 1, updated_at = NOW() WHERE id = $1`, messageID); err != nil {
			return fmt.Errorf("increment message likes: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *MessageRepository) RemoveReaction(ctx context.Context, messageID, userID int64, emoji string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cmd, err := tx.Exec(ctx,
		`DELETE FROM message_reactions WHERE message_id=$1 AND user_id=$2 AND emoji=$3`,
		messageID, userID, emoji)
	if err != nil {
		return fmt.Errorf("delete message reaction: %w", err)
	}
	if cmd.RowsAffected() > 0 {
		if _, err := tx.Exec(ctx, `UPDATE messages SET likes_count = GREATEST(0, likes_count - 1), updated_at = NOW() WHERE id = $1`, messageID); err != nil {
			return fmt.Errorf("decrement message likes: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *MessageRepository) GetReactionsSummaryBatch(ctx context.Context, messageIDs []int64, viewerID int64) (map[int64][]domainmessenger.ReactionSummary, error) {
	out := make(map[int64][]domainmessenger.ReactionSummary)
	if len(messageIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT message_id, emoji, COUNT(*),
		       BOOL_OR(user_id = $2) AS reacted_by_me
		FROM message_reactions
		WHERE message_id = ANY($1)
		GROUP BY message_id, emoji
		ORDER BY message_id, emoji`,
		messageIDs, viewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var msgID int64
		var s domainmessenger.ReactionSummary
		var count int
		if err := rows.Scan(&msgID, &s.Emoji, &count, &s.ReactedByMe); err != nil {
			return nil, err
		}
		s.Count = count
		out[msgID] = append(out[msgID], s)
	}
	return out, nil
}

func (r *MessageRepository) GetReactionsSummary(ctx context.Context, messageID, viewerID int64) ([]domainmessenger.ReactionSummary, error) {
	m, err := r.GetReactionsSummaryBatch(ctx, []int64{messageID}, viewerID)
	if err != nil {
		return nil, err
	}
	return m[messageID], nil
}

func (r *MessageRepository) GetReaction(ctx context.Context, messageID, userID int64) (*domainmessenger.Reaction, error) {
	var rr domainmessenger.Reaction
	err := r.db.QueryRow(ctx, `SELECT message_id, user_id, created_at FROM message_reactions WHERE message_id=$1 AND user_id=$2`, messageID, userID).Scan(&rr.MessageID, &rr.UserID, &rr.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &rr, nil
}
func (r *MessageRepository) UpsertDeliveryStatus(ctx context.Context, s *domainmessenger.MessageDeliveryStatus) error {
	_, err := r.db.Exec(ctx, `INSERT INTO message_delivery_status (message_id, user_id, status, updated_at) VALUES ($1,$2,$3,$4) ON CONFLICT (message_id, user_id) DO UPDATE SET status=EXCLUDED.status, updated_at=EXCLUDED.updated_at`, s.MessageID, s.UserID, s.Status, s.UpdatedAt)
	return err
}
func (r *MessageRepository) GetDeliveryStatuses(ctx context.Context, messageID int64) ([]domainmessenger.MessageDeliveryStatus, error) {
	rows, err := r.db.Query(ctx, `SELECT message_id, user_id, status, updated_at FROM message_delivery_status WHERE message_id=$1`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domainmessenger.MessageDeliveryStatus{}
	for rows.Next() {
		var s domainmessenger.MessageDeliveryStatus
		if err := rows.Scan(&s.MessageID, &s.UserID, &s.Status, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}
func (r *MessageRepository) ListExpiredDisappearing(ctx context.Context) ([]*domainmessenger.Message, error) {
	rows, err := r.db.Query(ctx, `SELECT id, conversation_id, sender_id, type, body, reply_to_id, forwarded_from_id, is_deleted, is_edited, edited_at, pinned, likes_count, disappears_at, scheduled_at, is_scheduled, delivery_status, mention_user_ids, created_at, updated_at FROM messages WHERE disappears_at IS NOT NULL AND disappears_at <= NOW() AND is_deleted=FALSE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domainmessenger.Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}
func (r *MessageRepository) HardDeleteByID(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM messages WHERE id=$1`, id)
	return err
}

func (r *MessageRepository) ListScheduled(ctx context.Context, convID, userID int64) ([]*domainmessenger.Message, error) {
	rows, err := r.db.Query(ctx, `SELECT id, conversation_id, sender_id, type, body, reply_to_id, forwarded_from_id, is_deleted, is_edited, edited_at, pinned, likes_count, disappears_at, scheduled_at, is_scheduled, delivery_status, mention_user_ids, created_at, updated_at FROM messages WHERE conversation_id = $1 AND sender_id = $2 AND is_scheduled = TRUE AND is_deleted = FALSE ORDER BY scheduled_at ASC`, convID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domainmessenger.Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}
func (r *MessageRepository) ListDueScheduled(ctx context.Context) ([]*domainmessenger.Message, error) {
	rows, err := r.db.Query(ctx, `SELECT id, conversation_id, sender_id, type, body, reply_to_id, forwarded_from_id, is_deleted, is_edited, edited_at, pinned, likes_count, disappears_at, scheduled_at, is_scheduled, delivery_status, mention_user_ids, created_at, updated_at FROM messages WHERE is_scheduled=TRUE AND scheduled_at <= NOW()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domainmessenger.Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}
func (r *MessageRepository) MarkScheduledSent(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE messages SET is_scheduled=FALSE, updated_at=NOW() WHERE id=$1`, id)
	return err
}
func (r *MessageRepository) ListMentions(ctx context.Context, userID int64, page pagination.Query) ([]*domainmessenger.Message, int64, error) {
	var total int64
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE $1 = ANY(mention_user_ids)`, userID).Scan(&total)
	rows, err := r.db.Query(ctx, `SELECT id, conversation_id, sender_id, type, body, reply_to_id, forwarded_from_id, is_deleted, is_edited, edited_at, pinned, likes_count, disappears_at, scheduled_at, is_scheduled, delivery_status, mention_user_ids, created_at, updated_at FROM messages WHERE $1 = ANY(mention_user_ids) ORDER BY created_at DESC LIMIT $2 OFFSET $3`, userID, page.Limit, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*domainmessenger.Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	return out, total, nil
}

func (r *MemberRepository) Add(ctx context.Context, m *domainmessenger.ConversationMember) error {
	_, err := r.db.Exec(ctx, `INSERT INTO conversation_members (conversation_id,user_id,role,joined_at,left_at,muted_until,last_read_message_id,last_read_at,is_archived) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, m.ConversationID, m.UserID, m.Role, m.JoinedAt, m.LeftAt, m.MutedUntil, m.LastReadMessageID, m.LastReadAt, m.IsArchived)
	return err
}
func (r *MemberRepository) Remove(ctx context.Context, convID, userID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE conversation_members SET left_at=NOW() WHERE conversation_id=$1 AND user_id=$2`, convID, userID)
	return err
}
func (r *MemberRepository) GetMember(ctx context.Context, convID, userID int64) (*domainmessenger.ConversationMember, error) {
	m, err := scanMember(r.db.QueryRow(ctx, `SELECT conversation_id, user_id, role, joined_at, left_at, muted_until, last_read_message_id, last_read_at, is_archived FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`, convID, userID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrMemberNotFound
		}
		return nil, err
	}
	return m, nil
}
func (r *MemberRepository) ListMembers(ctx context.Context, convID int64) ([]*domainmessenger.ConversationMember, error) {
	rows, err := r.db.Query(ctx, `SELECT conversation_id, user_id, role, joined_at, left_at, muted_until, last_read_message_id, last_read_at, is_archived FROM conversation_members WHERE conversation_id=$1 AND left_at IS NULL`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domainmessenger.ConversationMember{}
	for rows.Next() {
		var m domainmessenger.ConversationMember
		var leftAt, mutedUntil, lastReadAt *time.Time
		var lastReadID *int64
		if err := rows.Scan(&m.ConversationID, &m.UserID, &m.Role, &m.JoinedAt, &leftAt, &mutedUntil, &lastReadID, &lastReadAt, &m.IsArchived); err != nil {
			return nil, err
		}
		m.LeftAt = leftAt
		m.MutedUntil = mutedUntil
		m.LastReadMessageID = lastReadID
		m.LastReadAt = lastReadAt
		out = append(out, &m)
	}
	return out, nil
}
func (r *MemberRepository) UpdateRole(ctx context.Context, convID, userID int64, role domainmessenger.MemberRole) error {
	_, err := r.db.Exec(ctx, `UPDATE conversation_members SET role=$3 WHERE conversation_id=$1 AND user_id=$2`, convID, userID, role)
	return err
}
func (r *MemberRepository) MarkRead(ctx context.Context, convID, userID, lastMsgID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE conversation_members SET last_read_message_id=$3, last_read_at=NOW() WHERE conversation_id=$1 AND user_id=$2`, convID, userID, lastMsgID)
	return err
}
func (r *MemberRepository) UpdateMembersCount(ctx context.Context, convID int64, delta int) error {
	_, err := r.db.Exec(ctx, `UPDATE conversations SET members_count = GREATEST(0, members_count + $2), updated_at=NOW() WHERE id=$1`, convID, delta)
	return err
}
func (r *MemberRepository) SetArchived(ctx context.Context, convID, userID int64, archived bool) error {
	_, err := r.db.Exec(ctx, `UPDATE conversation_members SET is_archived=$3 WHERE conversation_id=$1 AND user_id=$2`, convID, userID, archived)
	return err
}

func (r *PresenceRepository) SetOnline(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `INSERT INTO user_presence (user_id, is_online, last_seen_at, updated_at) VALUES ($1, TRUE, NOW(), NOW()) ON CONFLICT (user_id) DO UPDATE SET is_online=TRUE, last_seen_at=NOW(), updated_at=NOW()`, userID)
	return err
}
func (r *PresenceRepository) SetOffline(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `INSERT INTO user_presence (user_id, is_online, last_seen_at, updated_at) VALUES ($1, FALSE, NOW(), NOW()) ON CONFLICT (user_id) DO UPDATE SET is_online=FALSE, last_seen_at=NOW(), updated_at=NOW()`, userID)
	return err
}
func (r *PresenceRepository) GetPresence(ctx context.Context, userID int64) (*domainmessenger.UserPresence, error) {
	var p domainmessenger.UserPresence
	err := r.db.QueryRow(ctx, `SELECT user_id, is_online, last_seen_at, updated_at FROM user_presence WHERE user_id=$1`, userID).Scan(&p.UserID, &p.IsOnline, &p.LastSeenAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}
func (r *PresenceRepository) GetPresenceBatch(ctx context.Context, userIDs []int64) (map[int64]*domainmessenger.UserPresence, error) {
	if len(userIDs) == 0 {
		return map[int64]*domainmessenger.UserPresence{}, nil
	}
	rows, err := r.db.Query(ctx, `SELECT user_id, is_online, last_seen_at, updated_at FROM user_presence WHERE user_id = ANY($1)`, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]*domainmessenger.UserPresence{}
	for rows.Next() {
		var p domainmessenger.UserPresence
		if err := rows.Scan(&p.UserID, &p.IsOnline, &p.LastSeenAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out[p.UserID] = &p
	}
	return out, nil
}

func (r *PrivacyRepository) Get(ctx context.Context, userID int64) (*domainmessenger.MessengerPrivacySettings, error) {
	var s domainmessenger.MessengerPrivacySettings
	err := r.db.QueryRow(ctx, `SELECT user_id, who_can_message, updated_at FROM messenger_privacy_settings WHERE user_id=$1`, userID).Scan(&s.UserID, &s.WhoCanMessage, &s.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrPrivacySettingsNotFound
		}
		return nil, err
	}
	return &s, nil
}
func (r *PrivacyRepository) Upsert(ctx context.Context, s *domainmessenger.MessengerPrivacySettings) error {
	_, err := r.db.Exec(ctx, `INSERT INTO messenger_privacy_settings (user_id, who_can_message, updated_at) VALUES ($1,$2,$3) ON CONFLICT (user_id) DO UPDATE SET who_can_message=EXCLUDED.who_can_message, updated_at=EXCLUDED.updated_at`, s.UserID, s.WhoCanMessage, s.UpdatedAt)
	return err
}
func (r *PrivacyRepository) Block(ctx context.Context, blockerID, blockedID int64) error {
	_, err := r.db.Exec(ctx, `INSERT INTO messenger_blocked_users (blocker_id, blocked_id, created_at) VALUES ($1,$2,NOW()) ON CONFLICT DO NOTHING`, blockerID, blockedID)
	return err
}
func (r *PrivacyRepository) Unblock(ctx context.Context, blockerID, blockedID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM messenger_blocked_users WHERE blocker_id=$1 AND blocked_id=$2`, blockerID, blockedID)
	return err
}
func (r *PrivacyRepository) IsBlocked(ctx context.Context, blockerID, blockedID int64) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messenger_blocked_users WHERE blocker_id=$1 AND blocked_id=$2)`, blockerID, blockedID).Scan(&ok)
	return ok, err
}
func (r *PrivacyRepository) ListBlocked(ctx context.Context, blockerID int64) ([]int64, error) {
	rows, err := r.db.Query(ctx, `SELECT blocked_id FROM messenger_blocked_users WHERE blocker_id=$1`, blockerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func (r *DraftRepository) Upsert(ctx context.Context, d *domainmessenger.MessageDraft) error {
	_, err := r.db.Exec(ctx, `INSERT INTO message_drafts (conversation_id, user_id, body, reply_to_id, updated_at) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (conversation_id, user_id) DO UPDATE SET body=EXCLUDED.body, reply_to_id=EXCLUDED.reply_to_id, updated_at=EXCLUDED.updated_at`, d.ConversationID, d.UserID, d.Body, d.ReplyToID, d.UpdatedAt)
	return err
}
func (r *DraftRepository) Get(ctx context.Context, convID, userID int64) (*domainmessenger.MessageDraft, error) {
	var d domainmessenger.MessageDraft
	err := r.db.QueryRow(ctx, `SELECT conversation_id, user_id, body, reply_to_id, updated_at FROM message_drafts WHERE conversation_id=$1 AND user_id=$2`, convID, userID).Scan(&d.ConversationID, &d.UserID, &d.Body, &d.ReplyToID, &d.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrDraftNotFound
		}
		return nil, err
	}
	return &d, nil
}
func (r *DraftRepository) Delete(ctx context.Context, convID, userID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM message_drafts WHERE conversation_id=$1 AND user_id=$2`, convID, userID)
	return err
}
func (r *DraftRepository) ListForUser(ctx context.Context, userID int64) ([]*domainmessenger.MessageDraft, error) {
	rows, err := r.db.Query(ctx, `SELECT conversation_id, user_id, body, reply_to_id, updated_at FROM message_drafts WHERE user_id=$1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domainmessenger.MessageDraft{}
	for rows.Next() {
		var d domainmessenger.MessageDraft
		if err := rows.Scan(&d.ConversationID, &d.UserID, &d.Body, &d.ReplyToID, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &d)
	}
	return out, nil
}

func (r *MessageRepository) GetLikedByMeBatch(ctx context.Context, userID int64, messageIDs []int64) (map[int64]bool, error) {
	if len(messageIDs) == 0 {
		return map[int64]bool{}, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT message_id FROM message_reactions WHERE user_id = $1 AND message_id = ANY($2)`,
		userID, messageIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, nil
}

func (r *MessageRepository) GetAttachmentsBatch(ctx context.Context, messageIDs []int64) (map[int64][]domainmessenger.Attachment, error) {
	if len(messageIDs) == 0 {
		return map[int64][]domainmessenger.Attachment{}, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, message_id, type, storage_key, url, mime_type, size_bytes, filename, duration_s, width, height, thumbnail_key, created_at
         FROM message_attachments WHERE message_id = ANY($1) ORDER BY id`,
		messageIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64][]domainmessenger.Attachment{}
	for rows.Next() {
		var a domainmessenger.Attachment
		if err := rows.Scan(&a.ID, &a.MessageID, &a.Type, &a.StorageKey, &a.URL, &a.MimeType, &a.SizeBytes, &a.Filename, &a.DurationS, &a.Width, &a.Height, &a.ThumbnailKey, &a.CreatedAt); err != nil {
			return nil, err
		}
		out[a.MessageID] = append(out[a.MessageID], a)
	}
	return out, nil
}

var (
	_ domainmessenger.ConversationRepository = (*ConversationRepository)(nil)
	_ domainmessenger.MessageRepository      = (*MessageRepository)(nil)
	_ domainmessenger.MemberRepository       = (*MemberRepository)(nil)
	_ domainmessenger.PresenceRepository     = (*PresenceRepository)(nil)
	_ domainmessenger.PrivacyRepository      = (*PrivacyRepository)(nil)
	_ domainmessenger.DraftRepository        = (*DraftRepository)(nil)
)

func _unused(_ ...any) {}

func init() {
	_ = json.RawMessage(nil)
	_ = mapPgErr
}
