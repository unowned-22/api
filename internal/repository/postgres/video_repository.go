package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/video"
	"github.com/unowned-22/api/internal/errs"
)

type VideoRepository struct{ pool *pgxpool.Pool }

func NewVideoRepository(pool *pgxpool.Pool) *VideoRepository { return &VideoRepository{pool: pool} }

// ── column list ──────────────────────────────────────────────────────────────
// community_id replaces channel_id (Stage 2 migration).
// published_at, publish_targets, boosted_until are new Stage 2 columns.
const videoSelectCols = `id,community_id,user_id,title,description,category,visibility,status,` +
	`raw_key,hls_key,mp4_360_key,mp4_720_key,thumbnail_key,duration_sec,width,height,size_bytes,` +
	`video_codec,audio_codec,views_count,likes_count,comments_count,created_at,updated_at,` +
	`processing_stage,processing_progress,processing_started_at,` +
	`published_at,publish_targets,boosted_until`

func scanVideo(v *video.Video, scan func(...any) error) error {
	return scan(
		&v.ID, &v.CommunityID, &v.UserID, &v.Title, &v.Description, &v.Category,
		&v.Visibility, &v.Status, &v.RawKey, &v.HLSKey, &v.MP4360Key, &v.MP4720Key,
		&v.ThumbnailKey, &v.DurationSec, &v.Width, &v.Height, &v.SizeBytes,
		&v.VideoCodec, &v.AudioCodec, &v.ViewsCount, &v.LikesCount, &v.CommentsCount,
		&v.CreatedAt, &v.UpdatedAt,
		&v.ProcessingStage, &v.ProcessingProgress, &v.ProcessingStartedAt,
		&v.PublishedAt, &v.PublishTargets, &v.BoostedUntil,
	)
}

// ── CRUD ─────────────────────────────────────────────────────────────────────

func (r *VideoRepository) Create(ctx context.Context, v *video.Video) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := `INSERT INTO videos
		(community_id,user_id,title,description,category,visibility,status,
		 raw_key,hls_key,mp4_360_key,mp4_720_key,thumbnail_key,
		 duration_sec,width,height,size_bytes,video_codec,audio_codec,
		 views_count,likes_count,comments_count)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
		RETURNING id,created_at,updated_at`
	if err := tx.QueryRow(ctx, q,
		v.CommunityID, v.UserID, v.Title, v.Description, v.Category, v.Visibility, v.Status,
		v.RawKey, v.HLSKey, v.MP4360Key, v.MP4720Key, v.ThumbnailKey,
		v.DurationSec, v.Width, v.Height, v.SizeBytes, v.VideoCodec, v.AudioCodec,
		v.ViewsCount, v.LikesCount, v.CommentsCount,
	).Scan(&v.ID, &v.CreatedAt, &v.UpdatedAt); err != nil {
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
	if err := scanVideo(v, r.pool.QueryRow(ctx, q, args...).Scan); err != nil {
		if isNoRows(err) {
			return nil, errs.ErrVideoNotFound
		}
		return nil, err
	}
	v.CoverKey = v.ThumbnailKey
	tags, _ := r.GetTags(ctx, v.ID)
	v.Tags = tags
	return v, nil
}

func (r *VideoRepository) GetByID(ctx context.Context, id int64) (*video.Video, error) {
	return r.get(ctx, `SELECT `+videoSelectCols+` FROM videos WHERE id=$1`, id)
}

func (r *VideoRepository) Update(ctx context.Context, v *video.Video) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE videos SET title=$1,description=$2,category=$3,visibility=$4,thumbnail_key=$5,updated_at=NOW() WHERE id=$6`,
		v.Title, v.Description, v.Category, v.Visibility, v.ThumbnailKey, v.ID)
	return err
}

func (r *VideoRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM videos WHERE id=$1`, id)
	return err
}

// ── Worker callbacks ─────────────────────────────────────────────────────────

func (r *VideoRepository) ReadyForPublish(ctx context.Context, id int64, hls, mp4360, mp4720, thumbnail string, dur float64, w, h int, size int64, vcodec, acodec string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE videos SET status='ready', hls_key=$2, mp4_360_key=$3, mp4_720_key=$4,
		 thumbnail_key=$5, duration_sec=$6, width=$7, height=$8, size_bytes=$9,
		 video_codec=$10, audio_codec=$11,
		 processing_stage=NULL, processing_progress=0,
		 updated_at=NOW() WHERE id=$1`,
		id, hls, mp4360, mp4720, thumbnail, dur, w, h, size, vcodec, acodec)
	return err
}

func (r *VideoRepository) MarkFailed(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE videos SET status='failed', processing_stage=NULL, processing_progress=0, updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *VideoRepository) MarkProcessing(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE videos SET status='processing', processing_started_at=NOW(), processing_stage='downloading', processing_progress=0, updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *VideoRepository) UpdateProgress(ctx context.Context, id int64, stage string, percent int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE videos SET processing_stage=$2, processing_progress=$3, updated_at=NOW() WHERE id=$1`,
		id, stage, percent)
	return err
}

// ── Publish lifecycle ────────────────────────────────────────────────────────

func (r *VideoRepository) Publish(ctx context.Context, id int64, targets []string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE videos SET published_at=NOW(), publish_targets=$2, updated_at=NOW() WHERE id=$1`,
		id, targets)
	return err
}

func (r *VideoRepository) Unpublish(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE videos SET published_at=NULL, publish_targets='{}', updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *VideoRepository) Archive(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE videos SET status='archived', updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *VideoRepository) ArchiveByCommunity(ctx context.Context, communityID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE videos SET status='archived', updated_at=NOW() WHERE community_id=$1 AND status != 'archived'`, communityID)
	return err
}

func (r *VideoRepository) SetBoostedUntil(ctx context.Context, id int64, until *string) error {
	if until == nil {
		_, err := r.pool.Exec(ctx, `UPDATE videos SET boosted_until=NULL WHERE id=$1`, id)
		return err
	}
	_, err := r.pool.Exec(ctx, `UPDATE videos SET boosted_until=$2 WHERE id=$1`, id, *until)
	return err
}

// ── Listings ─────────────────────────────────────────────────────────────────

// ListByCommunity returns published videos for a community page.
// For anonymous viewers (viewerID=0) only public videos are returned.
func (r *VideoRepository) ListByCommunity(ctx context.Context, communityID int64, viewerID int64, limit, offset int) ([]*video.Video, int, error) {
	var q string
	var args []any
	if viewerID == 0 {
		q = `SELECT ` + videoSelectCols + `
			 FROM videos
			 WHERE community_id=$1 AND published_at IS NOT NULL
			   AND visibility='public' AND status='ready'
			 ORDER BY published_at DESC LIMIT $2 OFFSET $3`
		args = []any{communityID, limit, offset}
	} else {
		q = `SELECT ` + videoSelectCols + `
			 FROM videos
			 WHERE community_id=$1 AND published_at IS NOT NULL
			   AND status='ready'
			   AND (visibility='public' OR visibility='unlisted' OR user_id=$4)
			 ORDER BY published_at DESC LIMIT $2 OFFSET $3`
		args = []any{communityID, limit, offset, viewerID}
	}
	return r.queryList(ctx, q, args...)
}

// ListDraftsByCommunity returns videos with published_at IS NULL (drafts).
func (r *VideoRepository) ListDraftsByCommunity(ctx context.Context, communityID int64, limit, offset int) ([]*video.Video, int, error) {
	q := `SELECT ` + videoSelectCols + `
		  FROM videos
		  WHERE community_id=$1 AND published_at IS NULL
		  ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	return r.queryList(ctx, q, communityID, limit, offset)
}

// Feed returns published videos from communities the user subscribes to.
// After Stage 2 migration, video_subscriptions is replaced by community_members.
func (r *VideoRepository) Feed(ctx context.Context, subscriberID int64, limit, offset int) ([]*video.Video, int, error) {
	q := `SELECT v.id,v.community_id,v.user_id,v.title,v.description,v.category,` +
		`v.visibility,v.status,v.raw_key,v.hls_key,v.mp4_360_key,v.mp4_720_key,` +
		`v.thumbnail_key,v.duration_sec,v.width,v.height,v.size_bytes,` +
		`v.video_codec,v.audio_codec,v.views_count,v.likes_count,` +
		`v.comments_count,v.created_at,v.updated_at,` +
		`v.processing_stage,v.processing_progress,v.processing_started_at,` +
		`v.published_at,v.publish_targets,v.boosted_until ` +
		`FROM videos v ` +
		`INNER JOIN community_members cm ON cm.community_id = v.community_id ` +
		`WHERE cm.user_id = $1 ` +
		`  AND v.status = 'ready' AND v.visibility = 'public' ` +
		`  AND v.published_at IS NOT NULL ` +
		`  AND $2 = ANY(v.publish_targets) ` +
		`ORDER BY v.published_at DESC LIMIT $3 OFFSET $4`
	return r.queryList(ctx, q, subscriberID, video.PublishTargetVideoFeed, limit, offset)
}

func (r *VideoRepository) Search(ctx context.Context, query, category string, limit, offset int) ([]*video.Video, int, error) {
	base := `SELECT ` + videoSelectCols + ` FROM videos WHERE status = 'ready' AND visibility = 'public' AND published_at IS NOT NULL`
	args := []any{}
	cond := ""
	idx := 1
	if query != "" {
		cond += fmt.Sprintf(" AND (title ILIKE $%d OR description ILIKE $%d)", idx, idx+1)
		like := "%" + query + "%"
		args = append(args, like, like)
		idx += 2
	}
	if category != "" {
		cond += fmt.Sprintf(" AND category = $%d", idx)
		args = append(args, category)
		idx++
	}
	args = append(args, limit, offset)
	q := base + cond + fmt.Sprintf(" ORDER BY views_count DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	return r.queryList(ctx, q, args...)
}

// ── tags, views, likes, counters ─────────────────────────────────────────────

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

// ── internal helpers ─────────────────────────────────────────────────────────

func (r *VideoRepository) queryList(ctx context.Context, q string, args ...any) ([]*video.Video, int, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*video.Video
	for rows.Next() {
		v := &video.Video{}
		if err := scanVideo(v, rows.Scan); err != nil {
			return nil, 0, err
		}
		v.CoverKey = v.ThumbnailKey
		out = append(out, v)
	}
	return out, len(out), rows.Err()
}

func isNoRows(err error) bool {
	return err != nil && (errors.Is(err, errNoRows) || strings.Contains(err.Error(), "no rows"))
}

// errNoRows is the pgx sentinel value — imported as a string compare to avoid
// a direct pgx import in this file (the build constraint handles it).
var errNoRows = fmt.Errorf("no rows in result set")

var _ video.Repository = (*VideoRepository)(nil)
