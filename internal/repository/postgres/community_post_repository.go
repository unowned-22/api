package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/communitypost"
	"github.com/unowned-22/api/internal/errs"
)

type CommunityPostRepository struct{ pool *pgxpool.Pool }

func NewCommunityPostRepository(pool *pgxpool.Pool) *CommunityPostRepository {
	return &CommunityPostRepository{pool: pool}
}

const communityPostSelectCols = `
	id, community_id, author_user_id, text, media, video_id, pinned,
	likes_count, comments_count, created_at, updated_at, deleted_at`

func scanCommunityPost(row pgx.Row) (*communitypost.Post, error) {
	p := &communitypost.Post{}
	var mediaRaw []byte
	err := row.Scan(
		&p.ID, &p.CommunityID, &p.AuthorUserID, &p.Text, &mediaRaw, &p.VideoID, &p.Pinned,
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

func (r *CommunityPostRepository) Create(ctx context.Context, p *communitypost.Post) error {
	mediaJSON, err := json.Marshal(p.Media)
	if err != nil {
		return err
	}
	return r.pool.QueryRow(ctx, `
		INSERT INTO community_posts (community_id, author_user_id, text, media, video_id, pinned)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, created_at, updated_at`,
		p.CommunityID, p.AuthorUserID, p.Text, mediaJSON, p.VideoID, p.Pinned,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *CommunityPostRepository) GetByID(ctx context.Context, id int64) (*communitypost.Post, error) {
	q := `SELECT ` + communityPostSelectCols + ` FROM community_posts WHERE id=$1 AND deleted_at IS NULL`
	return scanCommunityPost(r.pool.QueryRow(ctx, q, id))
}

func (r *CommunityPostRepository) Update(ctx context.Context, p *communitypost.Post) error {
	mediaJSON, err := json.Marshal(p.Media)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE community_posts
		   SET text=$1, media=$2, pinned=$3, updated_at=NOW()
		 WHERE id=$4`,
		p.Text, mediaJSON, p.Pinned, p.ID,
	)
	return err
}

func (r *CommunityPostRepository) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE community_posts SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *CommunityPostRepository) ListByCommunity(ctx context.Context, communityID int64, limit, offset int) ([]*communitypost.Post, error) {
	q := `SELECT ` + communityPostSelectCols + `
		  FROM community_posts WHERE community_id=$1 AND deleted_at IS NULL
		  ORDER BY pinned DESC, created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, q, communityID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*communitypost.Post
	for rows.Next() {
		p, err := scanCommunityPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateForVideo is called exclusively by the Stage 4 publish bridge
// (VideoService.Publish) — see communitypost.Repository doc comment.
func (r *CommunityPostRepository) CreateForVideo(ctx context.Context, communityID, authorUserID, videoID int64) (*communitypost.Post, error) {
	p := &communitypost.Post{
		CommunityID:  communityID,
		AuthorUserID: authorUserID,
		VideoID:      &videoID,
		Media:        []communitypost.MediaItem{},
	}
	if err := r.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (r *CommunityPostRepository) IncrLikesCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE community_posts SET likes_count=likes_count+1 WHERE id=$1`, id)
	return err
}

func (r *CommunityPostRepository) DecrLikesCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE community_posts SET likes_count=GREATEST(likes_count-1,0) WHERE id=$1`, id)
	return err
}

func (r *CommunityPostRepository) IncrCommentsCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE community_posts SET comments_count=comments_count+1 WHERE id=$1`, id)
	return err
}

func (r *CommunityPostRepository) DecrCommentsCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE community_posts SET comments_count=GREATEST(comments_count-1,0) WHERE id=$1`, id)
	return err
}

func (r *CommunityPostRepository) SoftDeleteByVideoID(ctx context.Context, videoID int64) (*int64, error) {
	var communityID int64
	err := r.pool.QueryRow(ctx, `
		UPDATE community_posts
		   SET deleted_at=NOW(), updated_at=NOW()
		 WHERE video_id=$1 AND deleted_at IS NULL
		RETURNING community_id`, videoID,
	).Scan(&communityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &communityID, nil
}

var _ communitypost.Repository = (*CommunityPostRepository)(nil)
