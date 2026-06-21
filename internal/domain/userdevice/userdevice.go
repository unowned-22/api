package userdevice

import (
	"context"
	"time"
)

// Device represents a known device for a user.
type Device struct {
	ID          int64
	UserID      int64
	Fingerprint string
	DeviceName  string
	Browser     string
	OS          string
	IP          string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}

// Repository defines persistence operations for user devices.
type Repository interface {
	// GetByFingerprint returns a device matching user_id and fingerprint, or ErrDeviceNotFound.
	GetByFingerprint(ctx context.Context, userID int64, fingerprint string) (*Device, error)
	// Create persists a new device record and sets its ID.
	Create(ctx context.Context, d *Device) error
	// UpdateLastSeen updates last_seen_at for an existing device.
	UpdateLastSeen(ctx context.Context, id int64, t time.Time) error
}
