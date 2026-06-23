package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/userprivacy"
)

type UserPrivacyRepository struct {
	db *pgxpool.Pool
}

func NewUserPrivacyRepository(db *pgxpool.Pool) *UserPrivacyRepository {
	return &UserPrivacyRepository{db: db}
}

func (r *UserPrivacyRepository) GetByUserID(ctx context.Context, userID int64) (*userprivacy.UserPrivacySettings, error) {
	query := `SELECT user_id, show_email, show_phone, show_friends, updated_at FROM user_privacy_settings WHERE user_id = $1`
	var s userprivacy.UserPrivacySettings
	err := r.db.QueryRow(ctx, query, userID).Scan(&s.UserID, &s.ShowEmail, &s.ShowPhone, &s.ShowFriends, &s.UpdatedAt)
	if err != nil {
		// assume not found -> return defaults
		return userprivacy.Default(userID), nil
	}
	return &s, nil
}

func (r *UserPrivacyRepository) Upsert(ctx context.Context, settings *userprivacy.UserPrivacySettings) error {
	query := `INSERT INTO user_privacy_settings (user_id, show_email, show_phone, show_friends, updated_at) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (user_id) DO UPDATE SET show_email = EXCLUDED.show_email, show_phone = EXCLUDED.show_phone, show_friends = EXCLUDED.show_friends, updated_at = EXCLUDED.updated_at`
	_, err := r.db.Exec(ctx, query, settings.UserID, settings.ShowEmail, settings.ShowPhone, settings.ShowFriends, time.Now())
	if err != nil {
		return fmt.Errorf("failed to upsert user privacy settings: %w", err)
	}
	return nil
}

// compile-time check
var _ userprivacy.Repository = (*UserPrivacyRepository)(nil)
