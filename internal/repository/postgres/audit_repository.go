package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/audit"
)

type AuditRepository struct {
	db *pgxpool.Pool
}

func NewAuditRepository(db *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(ctx context.Context, a *audit.AuditLog) error {
	metadata := []byte("null")
	if a.Metadata != nil {
		b, err := json.Marshal(a.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal audit metadata: %w", err)
		}
		metadata = b
	}

	query := `INSERT INTO audit_logs (user_id, event_type, ip_address, user_agent, metadata, created_at)
VALUES ($1, $2, $3, $4, $5::jsonb, $6) RETURNING id`

	var id int64
	err := r.db.QueryRow(ctx, query,
		a.UserID,
		a.EventType,
		a.IPAddress,
		a.UserAgent,
		string(metadata),
		time.Now(),
	).Scan(&id)
	if err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}
	a.ID = id
	return nil
}

var _ audit.Repository = (*AuditRepository)(nil)
