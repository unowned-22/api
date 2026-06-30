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

// Create inserts a new story row. Each publish creates an independent story.
func (r *StoryRepository) Create(ctx context.Context, s *story.Story) error {
	query := `
		INSERT INTO stories (user_id, visibility, duration_hours, hidden_from_user_ids, slides, expires_at, created_at, author_type, community_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query,
		s.UserID, s.Visibility, s.DurationHours,
		s.HiddenFromUserIDs, s.Slides, s.ExpiresAt, s.CreatedAt,
		s.AuthorType, s.CommunityID,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create story: %w", err)
	}
	return nil
}

func (r *StoryRepository) ListActiveByUser(ctx context.Context, userID int64) ([]*story.Story, error) {
	query := `
        SELECT id, user_id, visibility, duration_hours, hidden_from_user_ids, slides, created_at, expires_at, author_type, community_id
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
		if err := rows.Scan(&s.ID, &s.UserID, &s.Visibility, &s.DurationHours, &hidden, &slides, &s.CreatedAt, &s.ExpiresAt, &s.AuthorType, &s.CommunityID); err != nil {
			return nil, fmt.Errorf("failed to scan story row: %w", err)
		}
		s.HiddenFromUserIDs = hidden
		s.Slides = slides
		out = append(out, &s)
	}
	return out, nil
}

func (r *StoryRepository) ListFeed(ctx context.Context, viewerID int64) ([]*story.Story, error) {
	query := `
        SELECT id, user_id, visibility, duration_hours, hidden_from_user_ids,
               slides, created_at, expires_at, author_type, community_id
        FROM stories
        WHERE author_type = 'user'
          AND expires_at > now()
          AND NOT ($1 = ANY(hidden_from_user_ids))
        UNION ALL
        SELECT s.id, s.user_id, s.visibility, s.duration_hours,
               s.hidden_from_user_ids, s.slides, s.created_at, s.expires_at,
               s.author_type, s.community_id
        FROM stories s
        INNER JOIN community_members cm
                ON cm.community_id = s.community_id AND cm.user_id = $1
        WHERE s.author_type = 'community'
          AND s.expires_at > now()
        ORDER BY created_at DESC
    `
	rows, err := r.db.Query(ctx, query, viewerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list feed stories: %w", err)
	}
	defer rows.Close()

	out := make([]*story.Story, 0)
	for rows.Next() {
		var s story.Story
		var hidden []int64
		var slides []byte
		if err := rows.Scan(&s.ID, &s.UserID, &s.Visibility, &s.DurationHours, &hidden, &slides, &s.CreatedAt, &s.ExpiresAt, &s.AuthorType, &s.CommunityID); err != nil {
			return nil, fmt.Errorf("failed to scan story row: %w", err)
		}
		s.HiddenFromUserIDs = hidden
		s.Slides = slides
		out = append(out, &s)
	}
	return out, nil
}

func (r *StoryRepository) IsCloseFriend(ctx context.Context, ownerID, friendID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM close_friends WHERE owner_id=$1 AND friend_id=$2)`,
		ownerID, friendID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check close friend: %w", err)
	}
	return exists, nil
}

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
        SELECT id, user_id, visibility, duration_hours, hidden_from_user_ids, slides, created_at, expires_at, s.author_type, s.community_id
        FROM stories WHERE id = $1
    `
	var s story.Story
	var hidden []int64
	var slides []byte
	err := r.db.QueryRow(ctx, query, id).Scan(&s.ID, &s.UserID, &s.Visibility, &s.DurationHours, &hidden, &slides, &s.CreatedAt, &s.ExpiresAt, &s.AuthorType, &s.CommunityID)
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
func (r *StoryRepository) ListReplies(ctx context.Context, viewerID int64, storyID int64) ([]*story.Reply, error) {
	query := `SELECT id, story_id, viewer_id, message, created_at FROM story_replies WHERE story_id = $1 ORDER BY created_at ASC`
	// viewerID is currently unused by the repository; keep signature for service-level access checks
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

func (r *StoryRepository) ListByCommunity(ctx context.Context, communityID int64, limit, offset int) ([]*story.Story, error) {
	query := `
        SELECT s.id, s.user_id, s.visibility, s.duration_hours,
               s.hidden_from_user_ids, s.slides, s.created_at, s.expires_at,
               s.author_type, s.community_id
        FROM stories s
        WHERE s.community_id = $1
          AND s.author_type = 'community'
          AND s.expires_at > now()
        ORDER BY s.created_at DESC
        LIMIT $2 OFFSET $3
    `
	rows, err := r.db.Query(ctx, query, communityID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list community stories: %w", err)
	}
	defer rows.Close()

	out := make([]*story.Story, 0)
	for rows.Next() {
		var s story.Story
		var hidden []int64
		var slides []byte
		if err := rows.Scan(&s.ID, &s.UserID, &s.Visibility, &s.DurationHours,
			&hidden, &slides, &s.CreatedAt, &s.ExpiresAt,
			&s.AuthorType, &s.CommunityID); err != nil {
			return nil, fmt.Errorf("failed to scan community story row: %w", err)
		}
		s.HiddenFromUserIDs = hidden
		s.Slides = slides
		out = append(out, &s)
	}
	return out, nil
}
