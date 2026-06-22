package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/story"
	"github.com/unowned-22/api/internal/errs"
)

type StoryRepository struct {
	db *pgxpool.Pool
}

func NewStoryRepository(db *pgxpool.Pool) *StoryRepository {
	return &StoryRepository{db: db}
}

// Upsert inserts a new story row for the user or appends slides to the
// existing row for that user. Business rules enforced at service layer
// (e.g. max slides) — this method performs the DB-level upsert and
// replaces per-row metadata with the provided values.
func (r *StoryRepository) Upsert(ctx context.Context, s *story.Story) error {
	query := `
		INSERT INTO stories (user_id, visibility, duration_hours, hidden_from_user_ids, slides, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO UPDATE
		SET
			slides = (stories.slides || EXCLUDED.slides),
			expires_at = EXCLUDED.expires_at,
			visibility = EXCLUDED.visibility,
			duration_hours = EXCLUDED.duration_hours,
			hidden_from_user_ids = EXCLUDED.hidden_from_user_ids
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query, s.UserID, s.Visibility, s.DurationHours, s.HiddenFromUserIDs, s.Slides, s.ExpiresAt, s.CreatedAt).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to upsert story: %w", err)
	}
	return nil
}

func (r *StoryRepository) ListActiveByUser(ctx context.Context, userID int64) ([]*story.Story, error) {
	query := `
        SELECT id, user_id, visibility, duration_hours, hidden_from_user_ids, slides, created_at, expires_at
        FROM stories
        WHERE user_id = $1 AND expires_at > now()
        ORDER BY created_at DESC
    `
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list stories: %w", err)
	}
	defer rows.Close()

	out := make([]*story.Story, 0)
	for rows.Next() {
		var s story.Story
		var hidden []int64
		var slides []byte
		if err := rows.Scan(&s.ID, &s.UserID, &s.Visibility, &s.DurationHours, &hidden, &slides, &s.CreatedAt, &s.ExpiresAt); err != nil {
			return nil, fmt.Errorf("failed to scan story row: %w", err)
		}
		s.HiddenFromUserIDs = hidden
		s.Slides = slides
		out = append(out, &s)
	}
	return out, nil
}

// ListFeed returns active stories visible to viewerID. Current implementation
// excludes stories where viewerID appears in hidden_from_user_ids and only
// returns stories with expires_at > now().
func (r *StoryRepository) ListFeed(ctx context.Context, viewerID int64) ([]*story.Story, error) {
	query := `
		SELECT id, user_id, visibility, duration_hours, hidden_from_user_ids, slides, created_at, expires_at
		FROM stories
		WHERE expires_at > now()
		  AND (hidden_from_user_ids IS NULL OR NOT (hidden_from_user_ids @> $1::bigint[]))
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, []int64{viewerID})
	if err != nil {
		return nil, fmt.Errorf("failed to list feed stories: %w", err)
	}
	defer rows.Close()

	out := make([]*story.Story, 0)
	for rows.Next() {
		var s story.Story
		var hidden []int64
		var slides []byte
		if err := rows.Scan(&s.ID, &s.UserID, &s.Visibility, &s.DurationHours, &hidden, &slides, &s.CreatedAt, &s.ExpiresAt); err != nil {
			return nil, fmt.Errorf("failed to scan story row: %w", err)
		}
		s.HiddenFromUserIDs = hidden
		s.Slides = slides
		out = append(out, &s)
	}
	return out, nil
}

// AddView records that viewerID has viewed a slide in a story. slideIndex may
// be nil when the view is for the whole story.
func (r *StoryRepository) AddView(ctx context.Context, viewerID int64, storyID int64, slideIndex *int) error {
	query := `INSERT INTO story_views (viewer_id, story_id, slide_index) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	var idx interface{}
	if slideIndex == nil {
		idx = nil
	} else {
		idx = *slideIndex
	}
	_, err := r.db.Exec(ctx, query, viewerID, storyID, idx)
	if err != nil {
		return fmt.Errorf("failed to add story view: %w", err)
	}
	return nil
}

// ListViewsByViewer returns a map[story_id]map[slide_index]bool for quick lookup.
func (r *StoryRepository) ListViewsByViewer(ctx context.Context, viewerID int64) (map[int64]map[int]bool, error) {
	query := `SELECT story_id, slide_index FROM story_views WHERE viewer_id = $1`
	rows, err := r.db.Query(ctx, query, viewerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list story views: %w", err)
	}
	defer rows.Close()
	out := make(map[int64]map[int]bool)
	for rows.Next() {
		var sid int64
		var sidx *int
		if err := rows.Scan(&sid, &sidx); err != nil {
			return nil, fmt.Errorf("failed to scan view row: %w", err)
		}
		if _, ok := out[sid]; !ok {
			out[sid] = make(map[int]bool)
		}
		if sidx != nil {
			out[sid][*sidx] = true
		} else {
			// nil slide_index -> mark whole story as seen using index -1
			out[sid][-1] = true
		}
	}
	return out, nil
}

func (r *StoryRepository) GetByID(ctx context.Context, id int64) (*story.Story, error) {
	query := `
        SELECT id, user_id, visibility, duration_hours, hidden_from_user_ids, slides, created_at, expires_at
        FROM stories WHERE id = $1
    `
	var s story.Story
	var hidden []int64
	var slides []byte
	err := r.db.QueryRow(ctx, query, id).Scan(&s.ID, &s.UserID, &s.Visibility, &s.DurationHours, &hidden, &slides, &s.CreatedAt, &s.ExpiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrStoryNotFound
		}
		return nil, fmt.Errorf("failed to get story: %w", err)
	}
	s.HiddenFromUserIDs = hidden
	s.Slides = slides
	return &s, nil
}

func (r *StoryRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM stories WHERE id = $1`
	cmd, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete story: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrStoryNotFound
	}
	return nil
}

// AddLike records a like by viewer on a story (idempotent).
func (r *StoryRepository) AddLike(ctx context.Context, viewerID int64, storyID int64) error {
	query := `INSERT INTO story_likes (story_id, viewer_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(ctx, query, storyID, viewerID)
	if err != nil {
		return fmt.Errorf("failed to add like: %w", err)
	}
	return nil
}

// RemoveLike deletes a like by viewer on a story.
func (r *StoryRepository) RemoveLike(ctx context.Context, viewerID int64, storyID int64) error {
	query := `DELETE FROM story_likes WHERE story_id = $1 AND viewer_id = $2`
	_, err := r.db.Exec(ctx, query, storyID, viewerID)
	if err != nil {
		return fmt.Errorf("failed to remove like: %w", err)
	}
	return nil
}

// AddReply inserts a reply message for a story.
func (r *StoryRepository) AddReply(ctx context.Context, viewerID int64, storyID int64, message string) error {
	query := `INSERT INTO story_replies (story_id, viewer_id, message) VALUES ($1, $2, $3)`
	_, err := r.db.Exec(ctx, query, storyID, viewerID, message)
	if err != nil {
		return fmt.Errorf("failed to add reply: %w", err)
	}
	return nil
}

// ListReplies returns recent replies for a story.
func (r *StoryRepository) ListReplies(ctx context.Context, storyID int64) ([]*story.Reply, error) {
	query := `SELECT id, story_id, viewer_id, message, created_at FROM story_replies WHERE story_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, query, storyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list replies: %w", err)
	}
	defer rows.Close()

	out := make([]*story.Reply, 0)
	for rows.Next() {
		var rp story.Reply
		if err := rows.Scan(&rp.ID, &rp.StoryID, &rp.ViewerID, &rp.Message, &rp.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan reply: %w", err)
		}
		out = append(out, &rp)
	}
	return out, nil
}

// ListExpired returns stories whose expires_at is in the past or equal to now.
func (r *StoryRepository) ListExpired(ctx context.Context) ([]*story.Story, error) {
	query := `
		SELECT id, user_id, visibility, duration_hours, hidden_from_user_ids, slides, created_at, expires_at
		FROM stories
		WHERE expires_at <= now()
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list expired stories: %w", err)
	}
	defer rows.Close()

	out := make([]*story.Story, 0)
	for rows.Next() {
		var s story.Story
		var hidden []int64
		var slides []byte
		if err := rows.Scan(&s.ID, &s.UserID, &s.Visibility, &s.DurationHours, &hidden, &slides, &s.CreatedAt, &s.ExpiresAt); err != nil {
			return nil, fmt.Errorf("failed to scan story row: %w", err)
		}
		s.HiddenFromUserIDs = hidden
		s.Slides = slides
		out = append(out, &s)
	}
	return out, nil
}
