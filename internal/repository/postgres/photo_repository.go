package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/photo"
	"github.com/unowned-22/api/internal/errs"
)

type PhotoRepository struct {
	db *pgxpool.Pool
}

func NewPhotoRepository(db *pgxpool.Pool) *PhotoRepository {
	return &PhotoRepository{db: db}
}

func (r *PhotoRepository) Create(ctx context.Context, p *photo.Photo) error {
	q := `INSERT INTO photos (user_id, album_id, display_name, storage_key, url, size_bytes, width, height, mime_type, visibility, hidden_from, device_name, device_os, device_type, latitude, longitude, location_name, exif_data, likes_count, comments_count, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21) RETURNING id, created_at, updated_at`
	err := r.db.QueryRow(ctx, q,
		p.UserID,
		p.AlbumID,
		p.DisplayName,
		p.StorageKey,
		p.URL,
		p.SizeBytes,
		p.Width,
		p.Height,
		p.MimeType,
		p.Visibility,
		p.HiddenFrom,
		p.DeviceName,
		p.DeviceOS,
		p.DeviceType,
		p.Latitude,
		p.Longitude,
		p.LocationName,
		p.ExifData,
		p.LikesCount,
		p.CommentsCount,
		p.CreatedAt,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23503" {
				return errs.ErrAlbumNotFound
			}
		}
		return fmt.Errorf("failed to insert photo: %w", err)
	}
	return nil
}

func (r *PhotoRepository) GetByID(ctx context.Context, id int64) (*photo.Photo, error) {
	q := `SELECT id, user_id, album_id, display_name, storage_key, url, size_bytes, width, height, mime_type, visibility, hidden_from, device_name, device_os, device_type, latitude, longitude, location_name, exif_data, likes_count, comments_count, created_at, updated_at FROM photos WHERE id = $1`
	var p photo.Photo
	var hidden []int64
	var width, height *int
	var deviceName, deviceOS, deviceType *string
	var latitude, longitude *float64
	var locationName *string
	var exifData []byte
	err := r.db.QueryRow(ctx, q, id).Scan(&p.ID, &p.UserID, &p.AlbumID, &p.DisplayName, &p.StorageKey, &p.URL, &p.SizeBytes, &width, &height, &p.MimeType, &p.Visibility, &hidden, &deviceName, &deviceOS, &deviceType, &latitude, &longitude, &locationName, &exifData, &p.LikesCount, &p.CommentsCount, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrPhotoNotFound
		}
		return nil, fmt.Errorf("failed to get photo: %w", err)
	}
	p.Width = width
	p.Height = height
	p.HiddenFrom = hidden
	p.DeviceName = deviceName
	p.DeviceOS = deviceOS
	p.DeviceType = deviceType
	p.Latitude = latitude
	p.Longitude = longitude
	p.LocationName = locationName
	p.ExifData = exifData
	// likes/comments default 0 already handled by DB
	return &p, nil
}

func (r *PhotoRepository) GetByStorageKey(ctx context.Context, key string) (*photo.Photo, error) {
	q := `SELECT id, user_id, album_id, display_name, storage_key, url, size_bytes, width, height, mime_type, visibility, hidden_from, device_name, device_os, device_type, latitude, longitude, location_name, exif_data, created_at, updated_at FROM photos WHERE storage_key = $1`
	var p photo.Photo
	var hidden []int64
	var width, height *int
	var deviceName, deviceOS, deviceType *string
	var latitude, longitude *float64
	var locationName *string
	var exifData []byte
	err := r.db.QueryRow(ctx, q, key).Scan(&p.ID, &p.UserID, &p.AlbumID, &p.DisplayName, &p.StorageKey, &p.URL, &p.SizeBytes, &width, &height, &p.MimeType, &p.Visibility, &hidden, &deviceName, &deviceOS, &deviceType, &latitude, &longitude, &locationName, &exifData, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get photo by storage key: %w", err)
	}
	p.Width = width
	p.Height = height
	p.HiddenFrom = hidden
	p.DeviceName = deviceName
	p.DeviceOS = deviceOS
	p.DeviceType = deviceType
	p.Latitude = latitude
	p.Longitude = longitude
	p.LocationName = locationName
	p.ExifData = exifData
	return &p, nil
}

func (r *PhotoRepository) ListByUser(ctx context.Context, userID int64, viewerID int64, limit, offset int) ([]*photo.Photo, int, error) {
	// count
	countQ := `SELECT COUNT(*) FROM photos p WHERE p.user_id = $1 AND (
        p.user_id = $2 OR (
            p.visibility = 'everyone' AND NOT ($2 = ANY(p.hidden_from))
        )
    )`
	var total int
	if err := r.db.QueryRow(ctx, countQ, userID, viewerID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count photos: %w", err)
	}

	q := `SELECT id, user_id, album_id, display_name, storage_key, url, size_bytes, width, height, mime_type, visibility, hidden_from, device_name, device_os, device_type, latitude, longitude, location_name, exif_data, likes_count, comments_count, created_at, updated_at
        FROM photos p
        WHERE p.user_id = $1 AND (
            p.user_id = $2 OR (
                p.visibility = 'everyone' AND NOT ($2 = ANY(p.hidden_from))
            )
        )
        ORDER BY p.created_at DESC
        LIMIT $3 OFFSET $4`

	rows, err := r.db.Query(ctx, q, userID, viewerID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query photos: %w", err)
	}
	defer rows.Close()

	out := make([]*photo.Photo, 0)
	for rows.Next() {
		var p photo.Photo
		var hidden []int64
		var width, height *int
		var deviceName, deviceOS, deviceType *string
		var latitude, longitude *float64
		var locationName *string
		var exifData []byte
		if err := rows.Scan(&p.ID, &p.UserID, &p.AlbumID, &p.DisplayName, &p.StorageKey, &p.URL, &p.SizeBytes, &width, &height, &p.MimeType, &p.Visibility, &hidden, &deviceName, &deviceOS, &deviceType, &latitude, &longitude, &locationName, &exifData, &p.LikesCount, &p.CommentsCount, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan photo row: %w", err)
		}
		p.Width = width
		p.Height = height
		p.HiddenFrom = hidden
		p.DeviceName = deviceName
		p.DeviceOS = deviceOS
		p.DeviceType = deviceType
		p.Latitude = latitude
		p.Longitude = longitude
		p.LocationName = locationName
		p.ExifData = exifData
		out = append(out, &p)
	}
	return out, total, nil
}

func (r *PhotoRepository) ListByAlbum(ctx context.Context, albumID int64, viewerID int64, limit, offset int) ([]*photo.Photo, int, error) {
	countQ := `SELECT COUNT(*) FROM photos p WHERE p.album_id = $1 AND (
        p.user_id = $2 OR (
            p.visibility = 'everyone' AND NOT ($2 = ANY(p.hidden_from))
        )
    )`
	var total int
	if err := r.db.QueryRow(ctx, countQ, albumID, viewerID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count album photos: %w", err)
	}

	q := `SELECT id, user_id, album_id, display_name, storage_key, url, size_bytes, width, height, mime_type, visibility, hidden_from, device_name, device_os, device_type, latitude, longitude, location_name, exif_data, likes_count, comments_count, created_at, updated_at
        FROM photos p
        WHERE p.album_id = $1 AND (
            p.user_id = $2 OR (
                p.visibility = 'everyone' AND NOT ($2 = ANY(p.hidden_from))
            )
        )
        ORDER BY p.created_at DESC
        LIMIT $3 OFFSET $4`

	rows, err := r.db.Query(ctx, q, albumID, viewerID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query album photos: %w", err)
	}
	defer rows.Close()

	out := make([]*photo.Photo, 0)
	for rows.Next() {
		var p photo.Photo
		var hidden []int64
		var width, height *int
		var deviceName, deviceOS, deviceType *string
		var latitude, longitude *float64
		var locationName *string
		var exifData []byte
		if err := rows.Scan(&p.ID, &p.UserID, &p.AlbumID, &p.DisplayName, &p.StorageKey, &p.URL, &p.SizeBytes, &width, &height, &p.MimeType, &p.Visibility, &hidden, &deviceName, &deviceOS, &deviceType, &latitude, &longitude, &locationName, &exifData, &p.LikesCount, &p.CommentsCount, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan photo row: %w", err)
		}
		p.Width = width
		p.Height = height
		p.HiddenFrom = hidden
		p.DeviceName = deviceName
		p.DeviceOS = deviceOS
		p.DeviceType = deviceType
		p.Latitude = latitude
		p.Longitude = longitude
		p.LocationName = locationName
		p.ExifData = exifData
		out = append(out, &p)
	}
	return out, total, nil
}

func (r *PhotoRepository) UpdateMeta(ctx context.Context, id int64, displayName string, visibility photo.Visibility, hiddenFrom []int64) error {
	q := `UPDATE photos SET display_name = $1, visibility = $2, hidden_from = $3, updated_at = NOW() WHERE id = $4`
	cmd, err := r.db.Exec(ctx, q, displayName, visibility, hiddenFrom, id)
	if err != nil {
		return fmt.Errorf("failed to update photo meta: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrPhotoNotFound
	}
	return nil
}

func (r *PhotoRepository) AddLike(ctx context.Context, userID, photoID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `INSERT INTO photo_likes (photo_id, user_id) VALUES ($1,$2)`, photoID, userID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errs.ErrPhotoAlreadyLiked
		}
		return fmt.Errorf("insert photo like: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE photos SET likes_count = likes_count + 1 WHERE id = $1`, photoID); err != nil {
		return fmt.Errorf("inc photo likes: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *PhotoRepository) RemoveLike(ctx context.Context, userID, photoID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cmd, err := tx.Exec(ctx, `DELETE FROM photo_likes WHERE photo_id = $1 AND user_id = $2`, photoID, userID)
	if err != nil {
		return fmt.Errorf("delete photo like: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrPhotoNotLiked
	}
	if _, err := tx.Exec(ctx, `UPDATE photos SET likes_count = GREATEST(0, likes_count - 1) WHERE id = $1`, photoID); err != nil {
		return fmt.Errorf("dec photo likes: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *PhotoRepository) IsLiked(ctx context.Context, userID, photoID int64) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM photo_likes WHERE photo_id = $1 AND user_id = $2)`, photoID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("is liked: %w", err)
	}
	return exists, nil
}

func (r *PhotoRepository) GetURLsByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	q := `SELECT id, url FROM photos WHERE id = ANY($1)`
	rows, err := r.db.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get photo urls: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var url string
		if err := rows.Scan(&id, &url); err != nil {
			return nil, fmt.Errorf("failed to scan photo url row: %w", err)
		}
		out[id] = url
	}
	return out, nil
}

func (r *PhotoRepository) MoveToAlbum(ctx context.Context, id int64, albumID *int64) error {
	q := `UPDATE photos SET album_id = $1, updated_at = NOW() WHERE id = $2`
	cmd, err := r.db.Exec(ctx, q, albumID, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return errs.ErrAlbumNotFound
		}
		return fmt.Errorf("failed to move photo to album: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrPhotoNotFound
	}
	return nil
}

func (r *PhotoRepository) Delete(ctx context.Context, id int64) error {
	q := `DELETE FROM photos WHERE id = $1`
	cmd, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("failed to delete photo: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrPhotoNotFound
	}
	return nil
}
