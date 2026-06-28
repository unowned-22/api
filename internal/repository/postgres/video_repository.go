package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/video"
	"github.com/unowned-22/api/internal/errs"
)

type VideoRepository struct{ pool *pgxpool.Pool }

func NewVideoRepository(pool *pgxpool.Pool) *VideoRepository { return &VideoRepository{pool: pool} }

func (r *VideoRepository) Create(ctx context.Context, v *video.Video) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := `INSERT INTO videos (channel_id,user_id,title,description,category,visibility,status,raw_key,hls_key,mp4_360_key,mp4_720_key,thumbnail_key,duration_sec,width,height,size_bytes,video_codec,audio_codec,views_count,likes_count,comments_count)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21) RETURNING id,created_at,updated_at`
	if err := tx.QueryRow(ctx, q, v.ChannelID, v.UserID, v.Title, v.Description, v.Category, v.Visibility, v.Status, v.RawKey, v.HLSKey, v.MP4360Key, v.MP4720Key, v.ThumbnailKey, v.DurationSec, v.Width, v.Height, v.SizeBytes, v.VideoCodec, v.AudioCodec, v.ViewsCount, v.LikesCount, v.CommentsCount).Scan(&v.ID, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return err
	}
	if len(v.Tags) > 0 {
		for _, t := range v.Tags {
			if _, err := tx.Exec(ctx, `INSERT INTO video_tags (video_id, tag) VALUES ($1,$2)`, v.ID, strings.TrimSpace(t)); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func (r *VideoRepository) get(ctx context.Context, q string, args ...any) (*video.Video, error) {
	v := &video.Video{}
	err := r.pool.QueryRow(ctx, q, args...).Scan(&v.ID, &v.ChannelID, &v.UserID, &v.Title, &v.Description, &v.Category, &v.Visibility, &v.Status, &v.RawKey, &v.HLSKey, &v.MP4360Key, &v.MP4720Key, &v.ThumbnailKey, &v.DurationSec, &v.Width, &v.Height, &v.SizeBytes, &v.VideoCodec, &v.AudioCodec, &v.ViewsCount, &v.LikesCount, &v.CommentsCount, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	v.CoverKey = v.ThumbnailKey
	tags, _ := r.GetTags(ctx, v.ID)
	v.Tags = tags
	return v, nil
}
func (r *VideoRepository) GetByID(ctx context.Context, id int64) (*video.Video, error) {
	return r.get(ctx, `SELECT id,channel_id,user_id,title,description,category,visibility,status,raw_key,hls_key,mp4_360_key,mp4_720_key,thumbnail_key,duration_sec,width,height,size_bytes,video_codec,audio_codec,views_count,likes_count,comments_count,created_at,updated_at FROM videos WHERE id=$1`, id)
}
func (r *VideoRepository) Update(ctx context.Context, v *video.Video) error {
	_, err := r.pool.Exec(ctx, `UPDATE videos SET title=$1,description=$2,category=$3,visibility=$4,thumbnail_key=$5,updated_at=NOW() WHERE id=$6`, v.Title, v.Description, v.Category, v.Visibility, v.ThumbnailKey, v.ID)
	return err
}
func (r *VideoRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM videos WHERE id=$1`, id)
	return err
}
func (r *VideoRepository) ReadyForPublish(ctx context.Context, id int64, hls, mp4360, mp4720, thumbnail string, dur float64, w, h int, size int64, vcodec, acodec string) error {
	_, err := r.pool.Exec(ctx, `UPDATE videos SET status='ready', hls_key=$2, mp4_360_key=$3, mp4_720_key=$4, thumbnail_key=$5, duration_sec=$6, width=$7, height=$8, size_bytes=$9, video_codec=$10, audio_codec=$11, updated_at=NOW() WHERE id=$1`, id, hls, mp4360, mp4720, thumbnail, dur, w, h, size, vcodec, acodec)
	return err
}
func (r *VideoRepository) MarkFailed(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE videos SET status='failed', updated_at=NOW() WHERE id=$1`, id)
	return err
}
func (r *VideoRepository) MarkProcessing(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE videos SET status='processing', updated_at=NOW() WHERE id=$1`, id)
	return err
}
func (r *VideoRepository) ListByChannel(ctx context.Context, channelID int64, viewerID int64, limit, offset int) ([]*video.Video, int, error) {
	return r.list(ctx, `WHERE channel_id=$1`, channelID, limit, offset)
}
func (r *VideoRepository) Feed(ctx context.Context, subscriberID int64, limit, offset int) ([]*video.Video, int, error) {
	return r.list(ctx, `WHERE status='ready' AND visibility='public'`, nil, limit, offset)
}
func (r *VideoRepository) Search(ctx context.Context, query, category string, limit, offset int) ([]*video.Video, int, error) {
	return r.list(ctx, `WHERE status='ready' AND visibility='public'`, nil, limit, offset)
}
func (r *VideoRepository) list(ctx context.Context, where string, arg any, limit, offset int) ([]*video.Video, int, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,channel_id,user_id,title,description,category,visibility,status,raw_key,hls_key,mp4_360_key,mp4_720_key,thumbnail_key,duration_sec,width,height,size_bytes,video_codec,audio_codec,views_count,likes_count,comments_count,created_at,updated_at FROM videos `+where+` ORDER BY created_at DESC LIMIT $2 OFFSET $3`, arg, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*video.Video
	for rows.Next() {
		v := &video.Video{}
		_ = rows.Scan(&v.ID, &v.ChannelID, &v.UserID, &v.Title, &v.Description, &v.Category, &v.Visibility, &v.Status, &v.RawKey, &v.HLSKey, &v.MP4360Key, &v.MP4720Key, &v.ThumbnailKey, &v.DurationSec, &v.Width, &v.Height, &v.SizeBytes, &v.VideoCodec, &v.AudioCodec, &v.ViewsCount, &v.LikesCount, &v.CommentsCount, &v.CreatedAt, &v.UpdatedAt)
		v.CoverKey = v.ThumbnailKey
		out = append(out, v)
	}
	return out, len(out), rows.Err()
}
func (r *VideoRepository) SetTags(ctx context.Context, videoID int64, tags []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM video_tags WHERE video_id=$1`, videoID); err != nil {
		return err
	}
	for _, t := range tags {
		if _, err := tx.Exec(ctx, `INSERT INTO video_tags(video_id, tag) VALUES ($1,$2)`, videoID, strings.TrimSpace(t)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (r *VideoRepository) GetTags(ctx context.Context, videoID int64) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT tag FROM video_tags WHERE video_id=$1 ORDER BY tag`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}
func (r *VideoRepository) RecordView(ctx context.Context, videoID int64, userID *int64, ipHash string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO video_views (video_id,user_id,ip_hash) VALUES ($1,$2,$3)`, videoID, userID, ipHash)
	return err
}
func (r *VideoRepository) AddLike(ctx context.Context, userID, videoID int64) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO video_likes(video_id,user_id) VALUES ($1,$2)`, videoID, userID)
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return errs.ErrVideoAlreadyLiked
		}
		return err
	}
	_, err = r.pool.Exec(ctx, `UPDATE videos SET likes_count=likes_count+1 WHERE id=$1`, videoID)
	return err
}
func (r *VideoRepository) RemoveLike(ctx context.Context, userID, videoID int64) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM video_likes WHERE video_id=$1 AND user_id=$2`, videoID, userID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errs.ErrVideoNotLiked
	}
	_, err = r.pool.Exec(ctx, `UPDATE videos SET likes_count=GREATEST(likes_count-1,0) WHERE id=$1`, videoID)
	return err
}
func (r *VideoRepository) IsLiked(ctx context.Context, userID, videoID int64) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM video_likes WHERE video_id=$1 AND user_id=$2)`, videoID, userID).Scan(&ok)
	return ok, err
}
func (r *VideoRepository) IncrViewsCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE videos SET views_count=views_count+1 WHERE id=$1`, id)
	return err
}
func (r *VideoRepository) IncrCommentsCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE videos SET comments_count=comments_count+1 WHERE id=$1`, id)
	return err
}
func (r *VideoRepository) DecrCommentsCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE videos SET comments_count=GREATEST(comments_count-1,0) WHERE id=$1`, id)
	return err
}

var _ video.Repository = (*VideoRepository)(nil)
