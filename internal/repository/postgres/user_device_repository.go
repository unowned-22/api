package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/userdevice"
)

// UserDeviceRepository is the Postgres implementation of userdevice.Repository.
type UserDeviceRepository struct {
	db *pgxpool.Pool
}

// NewUserDeviceRepository creates a new repository.
func NewUserDeviceRepository(db *pgxpool.Pool) *UserDeviceRepository {
	return &UserDeviceRepository{db: db}
}

// GetByUnique finds a device by user_id + fingerprint + browser + country.
func (r *UserDeviceRepository) GetByUnique(userID int64, fingerprint, browser, country string) (*userdevice.Device, error) {
	ctx := context.Background()
	query := `SELECT id, user_id, fingerprint, browser, platform, country, city, ip, last_seen, created_at FROM user_devices WHERE user_id=$1 AND fingerprint=$2 AND browser=$3 AND country=$4`
	var d userdevice.Device
	err := r.db.QueryRow(ctx, query, userID, fingerprint, browser, country).
		Scan(&d.ID, &d.UserID, &d.Fingerprint, &d.Browser, &d.Platform, &d.Country, &d.City, &d.IP, &d.LastSeen, &d.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query user_devices: %w", err)
	}
	return &d, nil
}

// Create persists a new device.
func (r *UserDeviceRepository) Create(d *userdevice.Device) error {
	ctx := context.Background()
	query := `INSERT INTO user_devices (user_id, fingerprint, browser, platform, country, city, ip, last_seen, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`
	err := r.db.QueryRow(ctx, query, d.UserID, d.Fingerprint, d.Browser, d.Platform, d.Country, d.City, d.IP, d.LastSeen, d.CreatedAt).Scan(&d.ID)
	if err != nil {
		return fmt.Errorf("failed to insert user_device: %w", err)
	}
	return nil
}

// UpdateLastSeen updates the last_seen timestamp for a device.
func (r *UserDeviceRepository) UpdateLastSeen(id int64, t time.Time) error {
	ctx := context.Background()
	query := `UPDATE user_devices SET last_seen = $1 WHERE id = $2`
	cmd, err := r.db.Exec(ctx, query, t, id)
	if err != nil {
		return fmt.Errorf("failed to update last_seen: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("no device found to update last_seen")
	}
	return nil
}

// Compile-time check
var _ userdevice.Repository = (*UserDeviceRepository)(nil)
