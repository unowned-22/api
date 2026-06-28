package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/videochannel"
	"github.com/unowned-22/api/internal/errs"
)

type VideoChannelRepository struct{ pool *pgxpool.Pool }

func NewVideoChannelRepository(pool *pgxpool.Pool) *VideoChannelRepository {
	return &VideoChannelRepository{pool: pool}
}

func (r *VideoChannelRepository) Create(ctx context.Context, c *videochannel.Channel) error {
	q := `INSERT INTO video_channels (user_id,name,description,avatar_key,banner_key,subscribers_count,videos_count)
	VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id,created_at,updated_at`
	return r.pool.QueryRow(ctx, q, c.UserID, c.Name, c.Description, c.AvatarKey, c.BannerKey, c.SubscribersCount, c.VideosCount).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *VideoChannelRepository) GetByID(ctx context.Context, id int64) (*videochannel.Channel, error) {
	return r.get(ctx, "id=$1", id)
}
func (r *VideoChannelRepository) GetByUserID(ctx context.Context, userID int64) (*videochannel.Channel, error) {
	return r.get(ctx, "user_id=$1", userID)
}

func (r *VideoChannelRepository) get(ctx context.Context, cond string, arg any) (*videochannel.Channel, error) {
	q := `SELECT id,user_id,name,description,avatar_key,banner_key,subscribers_count,videos_count,created_at,updated_at FROM video_channels WHERE ` + cond
	c := &videochannel.Channel{}
	err := r.pool.QueryRow(ctx, q, arg).Scan(&c.ID, &c.UserID, &c.Name, &c.Description, &c.AvatarKey, &c.BannerKey, &c.SubscribersCount, &c.VideosCount, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, errs.ErrChannelNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *VideoChannelRepository) Update(ctx context.Context, c *videochannel.Channel) error {
	_, err := r.pool.Exec(ctx, `UPDATE video_channels SET name=$1,description=$2,avatar_key=$3,banner_key=$4,updated_at=NOW() WHERE id=$5`, c.Name, c.Description, c.AvatarKey, c.BannerKey, c.ID)
	return err
}
func (r *VideoChannelRepository) IncrVideosCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE video_channels SET videos_count = videos_count + 1 WHERE id=$1`, id)
	return err
}
func (r *VideoChannelRepository) DecrVideosCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE video_channels SET videos_count = GREATEST(videos_count - 1,0) WHERE id=$1`, id)
	return err
}
func (r *VideoChannelRepository) IncrSubscribers(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE video_channels SET subscribers_count = subscribers_count + 1 WHERE id=$1`, id)
	return err
}
func (r *VideoChannelRepository) DecrSubscribers(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE video_channels SET subscribers_count = GREATEST(subscribers_count - 1,0) WHERE id=$1`, id)
	return err
}

var _ videochannel.Repository = (*VideoChannelRepository)(nil)
