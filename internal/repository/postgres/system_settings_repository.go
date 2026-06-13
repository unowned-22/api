package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/systemsettings"
)

// SystemSettingsRepository implements systemsettings.Repository using Postgres.
type SystemSettingsRepository struct {
	db *pgxpool.Pool
}

func NewSystemSettingsRepository(pool *pgxpool.Pool) *SystemSettingsRepository {
	return &SystemSettingsRepository{db: pool}
}

func (r *SystemSettingsRepository) GetAll(ctx context.Context) ([]*systemsettings.Setting, error) {
	query := `SELECT key, value, updated_at FROM system_settings ORDER BY key`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query system_settings: %w", err)
	}
	defer rows.Close()

	var out []*systemsettings.Setting
	for rows.Next() {
		var key string
		var raw json.RawMessage
		var updatedAt time.Time
		if err := rows.Scan(&key, &raw, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan system_settings row: %w", err)
		}
		out = append(out, &systemsettings.Setting{Key: key, Value: raw, UpdatedAt: updatedAt})
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("row iteration error: %w", rows.Err())
	}
	return out, nil
}

func (r *SystemSettingsRepository) GetByKey(ctx context.Context, key string) (*systemsettings.Setting, error) {
	query := `SELECT key, value, updated_at FROM system_settings WHERE key = $1`
	var s systemsettings.Setting
	var raw json.RawMessage
	if err := r.db.QueryRow(ctx, query, key).Scan(&s.Key, &raw, &s.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get system setting by key: %w", err)
	}
	s.Value = raw
	return &s, nil
}

func (r *SystemSettingsRepository) Upsert(ctx context.Context, key string, value json.RawMessage) error {
	query := `
        INSERT INTO system_settings (key, value, updated_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
    `
	if _, err := r.db.Exec(ctx, query, key, value); err != nil {
		return fmt.Errorf("failed to upsert system setting: %w", err)
	}
	return nil
}
