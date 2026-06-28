package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/videosubscription"
	"github.com/unowned-22/api/internal/errs"
)

type VideoSubscriptionRepository struct{ pool *pgxpool.Pool }

func NewVideoSubscriptionRepository(pool *pgxpool.Pool) *VideoSubscriptionRepository {
	return &VideoSubscriptionRepository{pool: pool}
}
func (r *VideoSubscriptionRepository) Subscribe(ctx context.Context, subscriberID, channelID int64) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO video_subscriptions(subscriber_id,channel_id) VALUES($1,$2)`, subscriberID, channelID)
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return errs.ErrAlreadySubscribed
		}
		return err
	}
	_, err = r.pool.Exec(ctx, `UPDATE video_channels SET subscribers_count = subscribers_count + 1 WHERE id=$1`, channelID)
	return err
}
func (r *VideoSubscriptionRepository) Unsubscribe(ctx context.Context, subscriberID, channelID int64) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM video_subscriptions WHERE subscriber_id=$1 AND channel_id=$2`, subscriberID, channelID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errs.ErrNotSubscribed
	}
	_, err = r.pool.Exec(ctx, `UPDATE video_channels SET subscribers_count = GREATEST(subscribers_count - 1,0) WHERE id=$1`, channelID)
	return err
}
func (r *VideoSubscriptionRepository) IsSubscribed(ctx context.Context, subscriberID, channelID int64) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM video_subscriptions WHERE subscriber_id=$1 AND channel_id=$2)`, subscriberID, channelID).Scan(&ok)
	return ok, err
}
func (r *VideoSubscriptionRepository) ListSubscriberIDs(ctx context.Context, channelID int64) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT subscriber_id FROM video_subscriptions WHERE channel_id=$1 ORDER BY created_at DESC`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
func (r *VideoSubscriptionRepository) ListSubscribedChannelIDs(ctx context.Context, subscriberID int64) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT channel_id FROM video_subscriptions WHERE subscriber_id=$1 ORDER BY created_at DESC`, subscriberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

var _ videosubscription.Repository = (*VideoSubscriptionRepository)(nil)
