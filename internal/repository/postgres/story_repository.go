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

func (r *StoryRepository) Create(ctx context.Context, s *story.Story) error {
	query := `
        INSERT INTO stories (user_id, visibility, duration_hours, hidden_from_user_ids, slides, expires_at, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id, created_at
    `
	err := r.db.QueryRow(ctx, query, s.UserID, s.Visibility, s.DurationHours, s.HiddenFromUserIDs, s.Slides, s.ExpiresAt, s.CreatedAt).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create story: %w", err)
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
