package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/album"
	"github.com/unowned-22/api/internal/errs"
)

type AlbumRepository struct {
	db *pgxpool.Pool
}

func NewAlbumRepository(db *pgxpool.Pool) *AlbumRepository {
	return &AlbumRepository{db: db}
}

func (r *AlbumRepository) Create(ctx context.Context, a *album.Album) error {
	q := `INSERT INTO albums (user_id, title, description, visibility, hidden_from, cover_photo_id, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, created_at, updated_at`
	if a.HiddenFrom == nil {
		a.HiddenFrom = make([]int64, 0)
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	err := r.db.QueryRow(ctx, q, a.UserID, a.Title, a.Description, a.Visibility, a.HiddenFrom, a.CoverPhotoID, a.CreatedAt).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return errs.ErrPhotoNotFound
		}
		return fmt.Errorf("failed to insert album: %w", err)
	}
	return nil
}

func (r *AlbumRepository) GetByID(ctx context.Context, id int64) (*album.Album, error) {
	q := `SELECT id, user_id, title, description, visibility, hidden_from, cover_photo_id, created_at, updated_at FROM albums WHERE id = $1`
	var a album.Album
	var hidden []int64
	var cover *int64
	err := r.db.QueryRow(ctx, q, id).Scan(&a.ID, &a.UserID, &a.Title, &a.Description, &a.Visibility, &hidden, &cover, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrAlbumNotFound
		}
		return nil, fmt.Errorf("failed to get album: %w", err)
	}
	a.HiddenFrom = hidden
	a.CoverPhotoID = cover
	return &a, nil
}

func (r *AlbumRepository) ListByUser(ctx context.Context, userID int64, viewerID int64, limit, offset int) ([]*album.Album, int, error) {
	countQ := `SELECT COUNT(*) FROM albums a WHERE a.user_id = $1 AND (
        a.user_id = $2 OR (
            a.visibility = 'everyone' AND NOT ($2 = ANY(a.hidden_from))
        )
    )`
	var total int
	if err := r.db.QueryRow(ctx, countQ, userID, viewerID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count albums: %w", err)
	}

	q := `SELECT id, user_id, title, description, visibility, hidden_from, cover_photo_id, created_at, updated_at
        FROM albums a
        WHERE a.user_id = $1 AND (
            a.user_id = $2 OR (
                a.visibility = 'everyone' AND NOT ($2 = ANY(a.hidden_from))
            )
        )
        ORDER BY a.created_at DESC
        LIMIT $3 OFFSET $4`

	rows, err := r.db.Query(ctx, q, userID, viewerID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query albums: %w", err)
	}
	defer rows.Close()

	out := make([]*album.Album, 0)
	for rows.Next() {
		var a album.Album
		var hidden []int64
		var cover *int64
		if err := rows.Scan(&a.ID, &a.UserID, &a.Title, &a.Description, &a.Visibility, &hidden, &cover, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan album row: %w", err)
		}
		a.HiddenFrom = hidden
		a.CoverPhotoID = cover
		out = append(out, &a)
	}
	return out, total, nil
}

func (r *AlbumRepository) Update(ctx context.Context, a *album.Album) error {
	q := `UPDATE albums SET title = $1, description = $2, visibility = $3, hidden_from = $4, updated_at = NOW() WHERE id = $5`
	if a.HiddenFrom == nil {
		a.HiddenFrom = make([]int64, 0)
	}
	cmd, err := r.db.Exec(ctx, q, a.Title, a.Description, a.Visibility, a.HiddenFrom, a.ID)
	if err != nil {
		return fmt.Errorf("failed to update album: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrAlbumNotFound
	}
	return nil
}

func (r *AlbumRepository) Delete(ctx context.Context, id int64) error {
	q := `DELETE FROM albums WHERE id = $1`
	cmd, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("failed to delete album: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrAlbumNotFound
	}
	return nil
}

func (r *AlbumRepository) SetCover(ctx context.Context, albumID int64, photoID *int64) error {
	q := `UPDATE albums SET cover_photo_id = $1, updated_at = NOW() WHERE id = $2`
	cmd, err := r.db.Exec(ctx, q, photoID, albumID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return errs.ErrPhotoNotFound
		}
		return fmt.Errorf("failed to set album cover: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrAlbumNotFound
	}
	return nil
}
