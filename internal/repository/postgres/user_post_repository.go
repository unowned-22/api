package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/userpost"
	"github.com/unowned-22/api/internal/errs"
)

type UserPostRepository struct{ pool *pgxpool.Pool }

func NewUserPostRepository(pool *pgxpool.Pool) *UserPostRepository {
	return &UserPostRepository{pool: pool}
}

const userPostSelectCols = `
	id, user_id, text, media, visibility,
	likes_count, comments_count, created_at, updated_at, deleted_at`

func scanUserPost(row pgx.Row) (*userpost.Post, error) {
	p := &userpost.Post{}
	var mediaRaw []byte
	err := row.Scan(
		&p.ID, &p.UserID, &p.Text, &mediaRaw, &p.Visibility,
		&p.LikesCount, &p.CommentsCount, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrPostNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(mediaRaw) > 0 {
		if err := json.Unmarshal(mediaRaw, &p.Media); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (r *UserPostRepository) Create(ctx context.Context, p *userpost.Post) error {
	mediaJSON, err := json.Marshal(p.Media)
	if err != nil {
		return err
	}
	return r.pool.QueryRow(ctx, `
		INSERT INTO user_posts (user_id, text, media, visibility)
		VALUES ($1,$2,$3,$4)
		RETURNING id, created_at, updated_at`,
		p.UserID, p.Text, mediaJSON, string(p.Visibility),
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *UserPostRepository) GetByID(ctx context.Context, id int64) (*userpost.Post, error) {
	q := `SELECT ` + userPostSelectCols + ` FROM user_posts WHERE id=$1 AND deleted_at IS NULL`
	return scanUserPost(r.pool.QueryRow(ctx, q, id))
}

func (r *UserPostRepository) Update(ctx context.Context, p *userpost.Post) error {
	mediaJSON, err := json.Marshal(p.Media)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE user_posts
		   SET text=$1, media=$2, visibility=$3, updated_at=NOW()
		 WHERE id=$4`,
		p.Text, mediaJSON, string(p.Visibility), p.ID,
	)
	return err
}

func (r *UserPostRepository) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE user_posts SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *UserPostRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*userpost.Post, error) {
	q := `SELECT ` + userPostSelectCols + `
		  FROM user_posts WHERE user_id=$1 AND deleted_at IS NULL
		  ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*userpost.Post
	for rows.Next() {
		p, err := scanUserPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *UserPostRepository) IncrLikesCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE user_posts SET likes_count=likes_count+1 WHERE id=$1`, id)
	return err
}

func (r *UserPostRepository) DecrLikesCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE user_posts SET likes_count=GREATEST(likes_count-1,0) WHERE id=$1`, id)
	return err
}

func (r *UserPostRepository) IncrCommentsCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE user_posts SET comments_count=comments_count+1 WHERE id=$1`, id)
	return err
}

func (r *UserPostRepository) DecrCommentsCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE user_posts SET comments_count=GREATEST(comments_count-1,0) WHERE id=$1`, id)
	return err
}

var _ userpost.Repository = (*UserPostRepository)(nil)
