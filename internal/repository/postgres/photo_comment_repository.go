package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/photocomment"
	"github.com/unowned-22/api/internal/errs"
)

type PhotoCommentRepository struct {
	db *pgxpool.Pool
}

func NewPhotoCommentRepository(db *pgxpool.Pool) *PhotoCommentRepository {
	return &PhotoCommentRepository{db: db}
}

func (r *PhotoCommentRepository) Create(ctx context.Context, c *photocomment.Comment) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := `INSERT INTO photo_comments (photo_id, author_id, parent_id, body) VALUES ($1,$2,$3,$4) RETURNING id, created_at, updated_at`
	if err := tx.QueryRow(ctx, q, c.PhotoID, c.AuthorID, c.ParentID, c.Body).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return fmt.Errorf("insert comment: %w", err)
	}

	// update parent's replies_count if parent provided
	if c.ParentID != nil {
		if _, err := tx.Exec(ctx, `UPDATE photo_comments SET replies_count = replies_count + 1 WHERE id = $1`, *c.ParentID); err != nil {
			return fmt.Errorf("inc parent replies: %w", err)
		}
	}

	// update photo's comments_count
	if _, err := tx.Exec(ctx, `UPDATE photos SET comments_count = comments_count + 1 WHERE id = $1`, c.PhotoID); err != nil {
		return fmt.Errorf("inc photo comments: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *PhotoCommentRepository) GetByID(ctx context.Context, id int64) (*photocomment.Comment, error) {
	q := `SELECT c.id, c.photo_id, c.author_id, c.parent_id, c.body, c.is_deleted, c.likes_count, c.replies_count, c.created_at, c.updated_at, u.id, u.full_name, u.username, u.avatar_url
        FROM photo_comments c
        JOIN users u ON u.id = c.author_id
        WHERE c.id = $1`
	var c photocomment.Comment
	var parent sql.NullInt64
	var authorID int64
	var authorID2 int64
	var fullName, username, avatar string
	if err := r.db.QueryRow(ctx, q, id).Scan(&c.ID, &c.PhotoID, &authorID, &parent, &c.Body, &c.IsDeleted, &c.LikesCount, &c.RepliesCount, &c.CreatedAt, &c.UpdatedAt, &authorID2, &fullName, &username, &avatar); err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrCommentNotFound
		}
		return nil, fmt.Errorf("select comment: %w", err)
	}
	if parent.Valid {
		v := parent.Int64
		c.ParentID = &v
	}
	c.AuthorID = authorID
	c.Author = &photocomment.Author{ID: authorID, FullName: fullName, Username: username, AvatarURL: avatar}
	return &c, nil
}

func (r *PhotoCommentRepository) ListRoots(ctx context.Context, photoID int64, viewerID int64, limit, offset int) ([]*photocomment.Comment, int, error) {
	// count
	countQ := `SELECT COUNT(*) FROM photo_comments c WHERE c.photo_id = $1 AND c.parent_id IS NULL`
	var total int
	if err := r.db.QueryRow(ctx, countQ, photoID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count roots: %w", err)
	}

	q := `SELECT c.id, c.photo_id, c.author_id, c.parent_id, c.body, c.is_deleted, c.likes_count, c.replies_count, c.created_at, c.updated_at, u.id, u.full_name, u.username, u.avatar_url,
        EXISTS(SELECT 1 FROM photo_comment_likes cl WHERE cl.comment_id = c.id AND cl.user_id = $3) AS is_liked
        FROM photo_comments c
        JOIN users u ON u.id = c.author_id
        WHERE c.photo_id = $1 AND c.parent_id IS NULL
        ORDER BY c.created_at ASC
        LIMIT $2 OFFSET $4`

	rows, err := r.db.Query(ctx, q, photoID, limit, viewerID, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query roots: %w", err)
	}
	defer rows.Close()

	out := make([]*photocomment.Comment, 0)
	for rows.Next() {
		var c photocomment.Comment
		var parent sql.NullInt64
		var authorID int64
		var fullName, username, avatar string
		var authorID2 int64
		if err := rows.Scan(&c.ID, &c.PhotoID, &c.AuthorID, &parent, &c.Body, &c.IsDeleted, &c.LikesCount, &c.RepliesCount, &c.CreatedAt, &c.UpdatedAt, &authorID2, &fullName, &username, &avatar, &c.IsLiked); err != nil {
			return nil, 0, fmt.Errorf("scan root: %w", err)
		}
		if parent.Valid {
			v := parent.Int64
			c.ParentID = &v
		}
		c.Author = &photocomment.Author{ID: authorID, FullName: fullName, Username: username, AvatarURL: avatar}
		out = append(out, &c)
	}
	return out, total, nil
}

func (r *PhotoCommentRepository) ListReplies(ctx context.Context, parentID int64, viewerID int64, limit, offset int) ([]*photocomment.Comment, int, error) {
	countQ := `SELECT COUNT(*) FROM photo_comments c WHERE c.parent_id = $1`
	var total int
	if err := r.db.QueryRow(ctx, countQ, parentID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count replies: %w", err)
	}

	q := `SELECT c.id, c.photo_id, c.author_id, c.parent_id, c.body, c.is_deleted, c.likes_count, c.replies_count, c.created_at, c.updated_at, u.id, u.full_name, u.username, u.avatar_url,
        EXISTS(SELECT 1 FROM photo_comment_likes cl WHERE cl.comment_id = c.id AND cl.user_id = $2) AS is_liked
        FROM photo_comments c
        JOIN users u ON u.id = c.author_id
        WHERE c.parent_id = $1
        ORDER BY c.created_at ASC
        LIMIT $3 OFFSET $4`

	rows, err := r.db.Query(ctx, q, parentID, viewerID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query replies: %w", err)
	}
	defer rows.Close()

	out := make([]*photocomment.Comment, 0)
	for rows.Next() {
		var c photocomment.Comment
		var parent sql.NullInt64
		var authorID int64
		var fullName, username, avatar string
		var authorID2 int64
		if err := rows.Scan(&c.ID, &c.PhotoID, &c.AuthorID, &parent, &c.Body, &c.IsDeleted, &c.LikesCount, &c.RepliesCount, &c.CreatedAt, &c.UpdatedAt, &authorID2, &fullName, &username, &avatar, &c.IsLiked); err != nil {
			return nil, 0, fmt.Errorf("scan reply: %w", err)
		}
		if parent.Valid {
			v := parent.Int64
			c.ParentID = &v
		}
		c.Author = &photocomment.Author{ID: authorID, FullName: fullName, Username: username, AvatarURL: avatar}
		out = append(out, &c)
	}
	return out, total, nil
}

func (r *PhotoCommentRepository) SoftDelete(ctx context.Context, id int64) error {
	// fetch comment to know photo_id and parent_id
	var photoID int64
	var parent sql.NullInt64
	var isDeleted bool
	err := r.db.QueryRow(ctx, `SELECT photo_id, parent_id, is_deleted FROM photo_comments WHERE id = $1`, id).Scan(&photoID, &parent, &isDeleted)
	if err != nil {
		if err == pgx.ErrNoRows {
			return errs.ErrCommentNotFound
		}
		return fmt.Errorf("select comment for delete: %w", err)
	}

	// perform update
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cmd, err := tx.Exec(ctx, `UPDATE photo_comments SET is_deleted = TRUE, body = '[удалено]', updated_at = NOW() WHERE id = $1 AND is_deleted = FALSE`, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrCommentAlreadyDeleted
	}

	// if root comment, decrement photos.comments_count
	var parentIDVal *int64
	if parent.Valid {
		v := parent.Int64
		parentIDVal = &v
	}
	if parentIDVal == nil {
		if _, err := tx.Exec(ctx, `UPDATE photos SET comments_count = GREATEST(0, comments_count - 1) WHERE id = $1`, photoID); err != nil {
			return fmt.Errorf("dec photo comments: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *PhotoCommentRepository) Update(ctx context.Context, id int64, body string) error {
	cmd, err := r.db.Exec(ctx, `UPDATE photo_comments SET body = $1, updated_at = NOW() WHERE id = $2 AND is_deleted = FALSE`, body, id)
	if err != nil {
		return fmt.Errorf("update comment: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrCommentNotFound
	}
	return nil
}

func (r *PhotoCommentRepository) AddLike(ctx context.Context, userID, commentID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `INSERT INTO photo_comment_likes (comment_id, user_id) VALUES ($1,$2)`, commentID, userID); err != nil {
		var pgErr *pgconn.PgError
		if err != nil && fmt.Errorf("%w", err) != nil {
			if ok := errors.As(err, &pgErr); ok && pgErr.Code == "23505" {
				return errs.ErrCommentAlreadyLiked
			}
		}
		return fmt.Errorf("insert comment like: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE photo_comments SET likes_count = likes_count + 1 WHERE id = $1`, commentID); err != nil {
		return fmt.Errorf("inc comment likes: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *PhotoCommentRepository) RemoveLike(ctx context.Context, userID, commentID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cmd, err := tx.Exec(ctx, `DELETE FROM photo_comment_likes WHERE comment_id = $1 AND user_id = $2`, commentID, userID)
	if err != nil {
		return fmt.Errorf("delete comment like: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrCommentNotLiked
	}

	if _, err := tx.Exec(ctx, `UPDATE photo_comments SET likes_count = GREATEST(0, likes_count - 1) WHERE id = $1`, commentID); err != nil {
		return fmt.Errorf("dec comment likes: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *PhotoCommentRepository) IsLiked(ctx context.Context, userID, commentID int64) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM photo_comment_likes WHERE comment_id = $1 AND user_id = $2)`, commentID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("is liked: %w", err)
	}
	return exists, nil
}
