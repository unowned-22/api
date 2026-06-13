package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/usersettings"
)

type UserSettingsRepository struct {
	db *pgxpool.Pool
}

func NewUserSettingsRepository(pool *pgxpool.Pool) *UserSettingsRepository {
	return &UserSettingsRepository{db: pool}
}

func (r *UserSettingsRepository) Create(ctx context.Context, s *usersettings.UserSettings) error {
	query := `
        INSERT INTO user_settings (user_id, storage_quota_bytes, storage_used_bytes, bucket_name, theme, updated_at)
        VALUES ($1, $2, $3, $4, $5, NOW())
    `
	if _, err := r.db.Exec(ctx, query, s.UserID, s.StorageQuotaBytes, s.StorageUsedBytes, s.BucketName, s.Theme); err != nil {
		return fmt.Errorf("failed to create user_settings: %w", err)
	}
	return nil
}

func (r *UserSettingsRepository) GetByUserID(ctx context.Context, userID int64) (*usersettings.UserSettings, error) {
	query := `SELECT user_id, storage_quota_bytes, storage_used_bytes, bucket_name, theme, updated_at FROM user_settings WHERE user_id = $1`
	var s usersettings.UserSettings
	var raw json.RawMessage
	if err := r.db.QueryRow(ctx, query, userID).Scan(&s.UserID, &s.StorageQuotaBytes, &s.StorageUsedBytes, &s.BucketName, &raw, &s.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query user_settings: %w", err)
	}
	s.Theme = raw
	return &s, nil
}

func (r *UserSettingsRepository) UpdateTheme(ctx context.Context, userID int64, theme json.RawMessage) error {
	query := `UPDATE user_settings SET theme = $1, updated_at = NOW() WHERE user_id = $2`
	cmd, err := r.db.Exec(ctx, query, theme, userID)
	if err != nil {
		return fmt.Errorf("failed to update theme: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("no user settings found to update theme")
	}
	return nil
}

func (r *UserSettingsRepository) UpdateQuota(ctx context.Context, userID int64, quotaBytes int64) error {
	query := `UPDATE user_settings SET storage_quota_bytes = $1, updated_at = NOW() WHERE user_id = $2`
	cmd, err := r.db.Exec(ctx, query, quotaBytes, userID)
	if err != nil {
		return fmt.Errorf("failed to update quota: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("no user settings found to update quota")
	}
	return nil
}

func (r *UserSettingsRepository) UpdateBucketName(ctx context.Context, userID int64, bucketName string) error {
	query := `UPDATE user_settings SET bucket_name = $1, updated_at = NOW() WHERE user_id = $2`
	cmd, err := r.db.Exec(ctx, query, bucketName, userID)
	if err != nil {
		return fmt.Errorf("failed to update bucket name: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("no user settings found to update bucket name")
	}
	return nil
}

func (r *UserSettingsRepository) IncrementUsedBytes(ctx context.Context, userID int64, delta int64) error {
	query := `UPDATE user_settings SET storage_used_bytes = storage_used_bytes + $1, updated_at = NOW() WHERE user_id = $2`
	cmd, err := r.db.Exec(ctx, query, delta, userID)
	if err != nil {
		return fmt.Errorf("failed to increment used bytes: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("no user settings found to increment used bytes")
	}
	return nil
}
