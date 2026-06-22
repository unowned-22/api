package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/pagination"
)

type NotificationRepository struct {
	db *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{db: pool}
}

func (r *NotificationRepository) Create(ctx context.Context, n *notification.Notification) error {
	query := `INSERT INTO notifications (user_id, actor_id, type, entity_type, entity_id, payload, is_read, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`
	var id int64
	if err := r.db.QueryRow(ctx, query, n.UserID, n.ActorID, string(n.Type), n.EntityType, n.EntityID, n.Payload, n.IsRead, n.CreatedAt).Scan(&id); err != nil {
		return fmt.Errorf("failed to insert notification: %w", err)
	}
	n.ID = id
	return nil
}

func (r *NotificationRepository) CreateBatch(ctx context.Context, ns []*notification.Notification) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	stmt := `INSERT INTO notifications (user_id, actor_id, type, entity_type, entity_id, payload, is_read, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`
	for _, n := range ns {
		var id int64
		if err := tx.QueryRow(ctx, stmt, n.UserID, n.ActorID, string(n.Type), n.EntityType, n.EntityID, n.Payload, n.IsRead, n.CreatedAt).Scan(&id); err != nil {
			return fmt.Errorf("failed to insert batch notification: %w", err)
		}
		n.ID = id
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (r *NotificationRepository) ListByUser(ctx context.Context, userID int64, page pagination.Query) ([]*notification.Notification, int64, error) {
	totalQ := `SELECT COUNT(*) FROM notifications WHERE user_id = $1`
	var total int64
	if err := r.db.QueryRow(ctx, totalQ, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	q := `SELECT id, user_id, actor_id, type, entity_type, entity_id, payload, is_read, created_at FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, userID, page.Limit, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	var res []*notification.Notification
	for rows.Next() {
		var n notification.Notification
		var t string
		var payload json.RawMessage
		if err := rows.Scan(&n.ID, &n.UserID, &n.ActorID, &t, &n.EntityType, &n.EntityID, &payload, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		n.Type = notification.Type(t)
		n.Payload = payload
		res = append(res, &n)
	}
	return res, total, nil
}

func (r *NotificationRepository) MarkRead(ctx context.Context, userID int64, notificationID int64) error {
	q := `UPDATE notifications SET is_read = TRUE WHERE id = $1 AND user_id = $2`
	cmd, err := r.db.Exec(ctx, q, notificationID, userID)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID int64) error {
	q := `UPDATE notifications SET is_read = TRUE WHERE user_id = $1 AND is_read = FALSE`
	if _, err := r.db.Exec(ctx, q, userID); err != nil {
		return fmt.Errorf("mark all read: %w", err)
	}
	return nil
}

func (r *NotificationRepository) CountUnread(ctx context.Context, userID int64) (int64, error) {
	q := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`
	var c int64
	if err := r.db.QueryRow(ctx, q, userID).Scan(&c); err != nil {
		return 0, fmt.Errorf("count unread: %w", err)
	}
	return c, nil
}
