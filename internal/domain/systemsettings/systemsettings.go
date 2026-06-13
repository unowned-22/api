package systemsettings

import (
	"context"
	"encoding/json"
	"time"
)

// Setting represents a key/value system setting persisted as JSON.
type Setting struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// Repository defines persistence operations for system settings.
type Repository interface {
	GetAll(ctx context.Context) ([]*Setting, error)
	GetByKey(ctx context.Context, key string) (*Setting, error)
	Upsert(ctx context.Context, key string, value json.RawMessage) error
}

// Service defines application-level operations for system settings.
type Service interface {
	GetAll(ctx context.Context) ([]*Setting, error)
	GetByKey(ctx context.Context, key string) (*Setting, error)
	Update(ctx context.Context, key string, value json.RawMessage) error
}
