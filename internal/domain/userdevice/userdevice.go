package userdevice

import (
	"context"
	"time"
)

// Device represents a known device for a user.
type Device struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Fingerprint string    `json:"fingerprint"`
	Browser     string    `json:"browser"`
	Platform    string    `json:"platform"`
	Country     string    `json:"country"`
	City        string    `json:"city"`
	IP          string    `json:"ip"`
	LastSeen    time.Time `json:"last_seen"`
	CreatedAt   time.Time `json:"created_at"`
}

// Repository defines persistence operations for user devices.
type Repository interface {
	// GetByUnique returns a device matching unique identifying fields or nil + error.
	GetByUnique(ctx context.Context, userID int64, fingerprint, browser, country string) (*Device, error)
	// Create persists a new device record and sets its ID.
	Create(ctx context.Context, d *Device) error
	// UpdateLastSeen updates last_seen for an existing device.
	UpdateLastSeen(ctx context.Context, id int64, t time.Time) error
}
