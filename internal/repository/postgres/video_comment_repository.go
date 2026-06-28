package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/videocomment"
	"github.com/unowned-22/api/internal/errs"
)

type VideoCommentRepository struct{ pool *pgxpool.Pool }

func NewVideoCommentRepository(pool *pgxpool.Pool) *VideoCommentRepository {
	return &VideoCommentRepository{pool: pool}
}

func (r *VideoCommentRepository) Create(ctx context.Context, c *videocomment.Comment) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO video_comments (video_id,user_id,parent_id,body) VALUES ($1,$2,$3,$4) RETURNING id,likes_count,is_deleted,created_at,updated_at`, c.VideoID, c.UserID, c.ParentID, c.Body).Scan(&c.ID, &c.LikesCount, &c.IsDeleted, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE videos SET comments_count = comments_count + 1 WHERE id=$1`, c.VideoID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *VideoCommentRepository) GetByID(ctx context.Context, id int64) (*videocomment.Comment, error) {
	c := &videocomment.Comment{}
	err := r.pool.QueryRow(ctx, `SELECT id,video_id,user_id,parent_id,body,likes_count,is_deleted,created_at,updated_at FROM video_comments WHERE id=$1`, id).Scan(&c.ID, &c.VideoID, &c.UserID, &c.ParentID, &c.Body, &c.LikesCount, &c.IsDeleted, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, errs.ErrVideoCommentNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}
func (r *VideoCommentRepository) ListByVideo(ctx context.Context, videoID int64, limit, offset int) ([]*videocomment.Comment, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM video_comments WHERE video_id=$1 AND parent_id IS NULL AND is_deleted=FALSE`, videoID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT id,video_id,user_id,parent_id,body,likes_count,is_deleted,created_at,updated_at FROM video_comments WHERE video_id=$1 AND parent_id IS NULL AND is_deleted=FALSE ORDER BY created_at DESC LIMIT $2 OFFSET $3`, videoID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*videocomment.Comment
	for rows.Next() {
		c := &videocomment.Comment{}
		if err := rows.Scan(&c.ID, &c.VideoID, &c.UserID, &c.ParentID, &c.Body, &c.LikesCount, &c.IsDeleted, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}
func (r *VideoCommentRepository) ListReplies(ctx context.Context, parentID int64) ([]*videocomment.Comment, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,video_id,user_id,parent_id,body,likes_count,is_deleted,created_at,updated_at FROM video_comments WHERE parent_id=$1 AND is_deleted=FALSE ORDER BY created_at ASC`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*videocomment.Comment
	for rows.Next() {
		c := &videocomment.Comment{}
		if err := rows.Scan(&c.ID, &c.VideoID, &c.UserID, &c.ParentID, &c.Body, &c.LikesCount, &c.IsDeleted, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (r *VideoCommentRepository) SoftDelete(ctx context.Context, id int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var videoID int64
	if err := tx.QueryRow(ctx, `UPDATE video_comments SET is_deleted=TRUE, body='' WHERE id=$1 RETURNING video_id`, id).Scan(&videoID); err != nil {
		if err == pgx.ErrNoRows {
			return errs.ErrVideoCommentNotFound
		}
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE videos SET comments_count=GREATEST(comments_count-1,0) WHERE id=$1`, videoID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *VideoCommentRepository) AddLike(ctx context.Context, userID, commentID int64) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO video_comment_likes(comment_id,user_id) VALUES ($1,$2)`, commentID, userID)
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return errs.ErrVideoCommentAlreadyLiked
		}
		return err
	}
	_, err = r.pool.Exec(ctx, `UPDATE video_comments SET likes_count=likes_count+1 WHERE id=$1`, commentID)
	return err
}
func (r *VideoCommentRepository) RemoveLike(ctx context.Context, userID, commentID int64) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM video_comment_likes WHERE comment_id=$1 AND user_id=$2`, commentID, userID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errs.ErrVideoCommentNotLiked
	}
	_, err = r.pool.Exec(ctx, `UPDATE video_comments SET likes_count=GREATEST(likes_count-1,0) WHERE id=$1`, commentID)
	return err
}
func (r *VideoCommentRepository) IsLiked(ctx context.Context, userID, commentID int64) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM video_comment_likes WHERE comment_id=$1 AND user_id=$2)`, commentID, userID).Scan(&ok)
	return ok, err
}

var _ videocomment.Repository = (*VideoCommentRepository)(nil)
