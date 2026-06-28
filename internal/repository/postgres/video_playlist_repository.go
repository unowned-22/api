package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/videoplaylist"
	"github.com/unowned-22/api/internal/errs"
)

type VideoPlaylistRepository struct{ pool *pgxpool.Pool }

func NewVideoPlaylistRepository(pool *pgxpool.Pool) *VideoPlaylistRepository {
	return &VideoPlaylistRepository{pool: pool}
}
func (r *VideoPlaylistRepository) Create(ctx context.Context, p *videoplaylist.Playlist) error {
	return r.pool.QueryRow(ctx, `INSERT INTO video_playlists(user_id,title,description,visibility) VALUES($1,$2,$3,$4) RETURNING id,items_count,created_at,updated_at`, p.UserID, p.Title, p.Description, p.Visibility).Scan(&p.ID, &p.ItemsCount, &p.CreatedAt, &p.UpdatedAt)
}
func (r *VideoPlaylistRepository) GetByID(ctx context.Context, id int64) (*videoplaylist.Playlist, error) {
	p := &videoplaylist.Playlist{}
	if err := r.pool.QueryRow(ctx, `SELECT id,user_id,title,description,visibility,items_count,created_at,updated_at FROM video_playlists WHERE id=$1`, id).Scan(&p.ID, &p.UserID, &p.Title, &p.Description, &p.Visibility, &p.ItemsCount, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrPlaylistNotFound
		}
		return nil, errs.ErrPlaylistNotFound
	}
	return p, nil
}
func (r *VideoPlaylistRepository) ListByUser(ctx context.Context, userID int64) ([]*videoplaylist.Playlist, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,user_id,title,description,visibility,items_count,created_at,updated_at FROM video_playlists WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*videoplaylist.Playlist
	for rows.Next() {
		p := &videoplaylist.Playlist{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.Title, &p.Description, &p.Visibility, &p.ItemsCount, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (r *VideoPlaylistRepository) Update(ctx context.Context, p *videoplaylist.Playlist) error {
	_, err := r.pool.Exec(ctx, `UPDATE video_playlists SET title=$1,description=$2,visibility=$3,updated_at=NOW() WHERE id=$4`, p.Title, p.Description, p.Visibility, p.ID)
	return err
}
func (r *VideoPlaylistRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM video_playlists WHERE id=$1`, id)
	return err
}
func (r *VideoPlaylistRepository) AddItem(ctx context.Context, playlistID, videoID int64) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO video_playlist_items(playlist_id,video_id) VALUES($1,$2)`, playlistID, videoID)
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return errs.ErrPlaylistItemExists
		}
		return err
	}
	_, err = r.pool.Exec(ctx, `UPDATE video_playlists SET items_count = items_count + 1 WHERE id=$1`, playlistID)
	return err
}
func (r *VideoPlaylistRepository) RemoveItem(ctx context.Context, playlistID, videoID int64) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM video_playlist_items WHERE playlist_id=$1 AND video_id=$2`, playlistID, videoID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errs.ErrPlaylistItemNotFound
	}
	_, err = r.pool.Exec(ctx, `UPDATE video_playlists SET items_count = GREATEST(items_count - 1,0) WHERE id=$1`, playlistID)
	return err
}
func (r *VideoPlaylistRepository) ListItems(ctx context.Context, playlistID int64, limit, offset int) ([]*videoplaylist.PlaylistItem, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM video_playlist_items WHERE playlist_id=$1`, playlistID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT id,playlist_id,video_id,position,added_at FROM video_playlist_items WHERE playlist_id=$1 ORDER BY position ASC, added_at ASC LIMIT $2 OFFSET $3`, playlistID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*videoplaylist.PlaylistItem
	for rows.Next() {
		item := &videoplaylist.PlaylistItem{}
		if err := rows.Scan(&item.ID, &item.PlaylistID, &item.VideoID, &item.Position, &item.AddedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}
func (r *VideoPlaylistRepository) ItemExists(ctx context.Context, playlistID, videoID int64) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM video_playlist_items WHERE playlist_id=$1 AND video_id=$2)`, playlistID, videoID).Scan(&ok)
	return ok, err
}

var _ videoplaylist.Repository = (*VideoPlaylistRepository)(nil)
