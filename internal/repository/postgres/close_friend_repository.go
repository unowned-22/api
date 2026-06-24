package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/closefriend"
)

type CloseFriendRepository struct {
	db *pgxpool.Pool
}

func NewCloseFriendRepository(db *pgxpool.Pool) *CloseFriendRepository {
	return &CloseFriendRepository{db: db}
}

func (r *CloseFriendRepository) Add(ctx context.Context, ownerID, friendID int64) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO close_friends (owner_id, friend_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		ownerID, friendID,
	)
	if err != nil {
		return fmt.Errorf("failed to add close friend: %w", err)
	}
	return nil
}

func (r *CloseFriendRepository) Remove(ctx context.Context, ownerID, friendID int64) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM close_friends WHERE owner_id=$1 AND friend_id=$2`,
		ownerID, friendID,
	)
	if err != nil {
		return fmt.Errorf("failed to remove close friend: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return nil
	}
	return nil
}

func (r *CloseFriendRepository) List(ctx context.Context, ownerID int64) ([]int64, error) {
	rows, err := r.db.Query(ctx, `SELECT friend_id FROM close_friends WHERE owner_id=$1 ORDER BY friend_id`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list close friends: %w", err)
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var friendID int64
		if err := rows.Scan(&friendID); err != nil {
			return nil, fmt.Errorf("failed to scan close friend: %w", err)
		}
		out = append(out, friendID)
	}
	return out, nil
}

func (r *CloseFriendRepository) IsCloseFriend(ctx context.Context, ownerID, friendID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM close_friends WHERE owner_id=$1 AND friend_id=$2)`,
		ownerID, friendID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check close friend: %w", err)
	}
	return exists, nil
}

var _ closefriend.Repository = (*CloseFriendRepository)(nil)
