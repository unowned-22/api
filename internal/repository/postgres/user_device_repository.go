package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/userdevice"
	"github.com/unowned-22/api/internal/errs"
)

// UserDeviceRepository is the PostgreSQL implementation of userdevice.Repository.
type UserDeviceRepository struct {
	db *pgxpool.Pool
}

// NewUserDeviceRepository creates a new PostgreSQL implementation of UserDeviceRepository.
func NewUserDeviceRepository(db *pgxpool.Pool) *UserDeviceRepository {
	return &UserDeviceRepository{db: db}
}

// GetByFingerprint returns the device matching user_id and fingerprint, or ErrDeviceNotFound.
func (r *UserDeviceRepository) GetByFingerprint(ctx context.Context, userID int64, fingerprint string) (*userdevice.Device, error) {
	query := `
		SELECT id, user_id, fingerprint, device_name, browser, os, ip, first_seen_at, last_seen_at
		FROM user_devices
		WHERE user_id = $1 AND fingerprint = $2
		LIMIT 1
	`
	var d userdevice.Device
	err := r.db.QueryRow(ctx, query, userID, fingerprint).
		Scan(&d.ID, &d.UserID, &d.Fingerprint, &d.DeviceName, &d.Browser, &d.OS, &d.IP, &d.FirstSeenAt, &d.LastSeenAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrDeviceNotFound
		}
		return nil, fmt.Errorf("failed to get device by fingerprint: %w", err)
	}
	return &d, nil
}

// Create persists a new device record and sets its ID.
func (r *UserDeviceRepository) Create(ctx context.Context, d *userdevice.Device) error {
	query := `
		INSERT INTO user_devices (user_id, fingerprint, device_name, browser, os, ip, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query,
		d.UserID, d.Fingerprint, d.DeviceName, d.Browser, d.OS, d.IP, d.FirstSeenAt, d.LastSeenAt,
	).Scan(&d.ID)
	if err != nil {
		return fmt.Errorf("failed to create device: %w", err)
	}
	return nil
}

// UpdateLastSeen updates last_seen_at for an existing device.
func (r *UserDeviceRepository) UpdateLastSeen(ctx context.Context, id int64, t time.Time) error {
	query := `UPDATE user_devices SET last_seen_at = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, t, id)
	if err != nil {
		return fmt.Errorf("failed to update device last_seen_at: %w", err)
	}
	return nil
}

// Compile-time check that UserDeviceRepository satisfies the domain contract.
var _ userdevice.Repository = (*UserDeviceRepository)(nil)
