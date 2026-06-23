package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/pagination"
)

type FriendshipRepository struct {
	db *pgxpool.Pool
}

func NewFriendshipRepository(db *pgxpool.Pool) *FriendshipRepository {
	return &FriendshipRepository{db: db}
}

func (r *FriendshipRepository) Create(ctx context.Context, requesterID, addresseeID int64) (*friendship.Friendship, error) {
	query := `INSERT INTO friendships (requester_id, addressee_id, status, created_at, updated_at) VALUES ($1,$2,$3, $4, $5) RETURNING id, created_at, updated_at`
	now := time.Now()
	var id int64
	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, query, requesterID, addresseeID, friendship.StatusPending, now, now).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "23505" {
				return nil, errs.ErrFriendshipAlreadyExist
			}
		}
		return nil, fmt.Errorf("failed to create friendship: %w", err)
	}
	return &friendship.Friendship{ID: id, RequesterID: requesterID, AddresseeID: addresseeID, Status: friendship.StatusPending, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}

func (r *FriendshipRepository) UpdateStatus(ctx context.Context, id int64, status friendship.Status) (*friendship.Friendship, error) {
	query := `UPDATE friendships SET status=$1, updated_at=now() WHERE id=$2 RETURNING id, requester_id, addressee_id, status, created_at, updated_at`
	var f friendship.Friendship
	err := r.db.QueryRow(ctx, query, status, id).Scan(&f.ID, &f.RequesterID, &f.AddresseeID, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrFriendshipNotFound
		}
		return nil, fmt.Errorf("failed to update friendship status: %w", err)
	}
	return &f, nil
}

func (r *FriendshipRepository) GetByUsers(ctx context.Context, userA, userB int64) (*friendship.Friendship, error) {
	query := `SELECT id, requester_id, addressee_id, status, created_at, updated_at FROM friendships WHERE (requester_id=$1 AND addressee_id=$2) OR (requester_id=$2 AND addressee_id=$1) LIMIT 1`
	var f friendship.Friendship
	err := r.db.QueryRow(ctx, query, userA, userB).Scan(&f.ID, &f.RequesterID, &f.AddresseeID, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get friendship by users: %w", err)
	}
	return &f, nil
}

func (r *FriendshipRepository) GetByID(ctx context.Context, id int64) (*friendship.Friendship, error) {
	query := `SELECT id, requester_id, addressee_id, status, created_at, updated_at FROM friendships WHERE id=$1`
	var f friendship.Friendship
	err := r.db.QueryRow(ctx, query, id).Scan(&f.ID, &f.RequesterID, &f.AddresseeID, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get friendship by id: %w", err)
	}
	return &f, nil
}

func (r *FriendshipRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM friendships WHERE id=$1`
	cmd, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete friendship: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrFriendshipNotFound
	}
	return nil
}

func (r *FriendshipRepository) ListFriends(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Friendship, int64, error) {
	offset := page.Offset()
	limit := page.Limit
	query := `SELECT id, requester_id, addressee_id, status, created_at, updated_at FROM friendships WHERE status='accepted' AND (requester_id=$1 OR addressee_id=$1) ORDER BY updated_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list friends: %w", err)
	}
	defer rows.Close()
	out := make([]*friendship.Friendship, 0)
	for rows.Next() {
		var f friendship.Friendship
		if err := rows.Scan(&f.ID, &f.RequesterID, &f.AddresseeID, &f.Status, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan friendship row: %w", err)
		}
		out = append(out, &f)
	}
	// total count
	var total int64
	cntQ := `SELECT count(1) FROM friendships WHERE status='accepted' AND (requester_id=$1 OR addressee_id=$1)`
	if err := r.db.QueryRow(ctx, cntQ, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count friends: %w", err)
	}
	return out, total, nil
}

func (r *FriendshipRepository) ListIncomingRequests(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Friendship, int64, error) {
	offset := page.Offset()
	limit := page.Limit
	query := `SELECT id, requester_id, addressee_id, status, created_at, updated_at FROM friendships WHERE addressee_id=$1 AND status='pending' ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list incoming requests: %w", err)
	}
	defer rows.Close()
	out := make([]*friendship.Friendship, 0)
	for rows.Next() {
		var f friendship.Friendship
		if err := rows.Scan(&f.ID, &f.RequesterID, &f.AddresseeID, &f.Status, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan friendship row: %w", err)
		}
		out = append(out, &f)
	}
	var total int64
	cntQ := `SELECT count(1) FROM friendships WHERE addressee_id=$1 AND status='pending'`
	if err := r.db.QueryRow(ctx, cntQ, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count incoming requests: %w", err)
	}
	return out, total, nil
}

func (r *FriendshipRepository) ListOutgoingRequests(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Friendship, int64, error) {
	offset := page.Offset()
	limit := page.Limit
	query := `SELECT id, requester_id, addressee_id, status, created_at, updated_at FROM friendships WHERE requester_id=$1 AND status='pending' ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list outgoing requests: %w", err)
	}
	defer rows.Close()
	out := make([]*friendship.Friendship, 0)
	for rows.Next() {
		var f friendship.Friendship
		if err := rows.Scan(&f.ID, &f.RequesterID, &f.AddresseeID, &f.Status, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan friendship row: %w", err)
		}
		out = append(out, &f)
	}
	var total int64
	cntQ := `SELECT count(1) FROM friendships WHERE requester_id=$1 AND status='pending'`
	if err := r.db.QueryRow(ctx, cntQ, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count outgoing requests: %w", err)
	}
	return out, total, nil
}

func (r *FriendshipRepository) IsFriend(ctx context.Context, userA, userB int64) (bool, error) {
	query := `SELECT count(1) FROM friendships WHERE ((requester_id=$1 AND addressee_id=$2) OR (requester_id=$2 AND addressee_id=$1)) AND status='accepted'`
	var cnt int64
	if err := r.db.QueryRow(ctx, query, userA, userB).Scan(&cnt); err != nil {
		return false, fmt.Errorf("failed to check isfriend: %w", err)
	}
	return cnt > 0, nil
}

func (r *FriendshipRepository) IsSubscriber(ctx context.Context, requesterID, addresseeID int64) (bool, error) {
	query := `SELECT count(1) FROM friendships WHERE requester_id=$1 AND addressee_id=$2 AND status IN ('pending','accepted')`
	var cnt int64
	if err := r.db.QueryRow(ctx, query, requesterID, addresseeID).Scan(&cnt); err != nil {
		return false, fmt.Errorf("failed to check issubscriber: %w", err)
	}
	return cnt > 0, nil
}

func (r *FriendshipRepository) GetFriendIDs(ctx context.Context, userID int64) ([]int64, error) {
	query := `SELECT CASE WHEN requester_id=$1 THEN addressee_id ELSE requester_id END as friend_id FROM friendships WHERE (requester_id=$1 OR addressee_id=$1) AND status='accepted'`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get friend ids: %w", err)
	}
	defer rows.Close()
	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan friend id: %w", err)
		}
		out = append(out, id)
	}
	return out, nil
}

func (r *FriendshipRepository) CountFriends(ctx context.Context, userID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM friendships WHERE (requester_id = $1 OR addressee_id = $1) AND status = 'accepted'`
	var cnt int64
	if err := r.db.QueryRow(ctx, query, userID).Scan(&cnt); err != nil {
		return 0, fmt.Errorf("failed to count friends: %w", err)
	}
	return cnt, nil
}

func (r *FriendshipRepository) ListSuggestions(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Suggestion, int64, error) {
	offset := page.Offset()
	limit := page.Limit

	query := `WITH excluded_ids AS (
		SELECT CASE WHEN requester_id = $1 THEN addressee_id ELSE requester_id END AS uid
		FROM friendships
		WHERE requester_id = $1 OR addressee_id = $1
	),
	my_friend_ids AS (
		SELECT CASE WHEN requester_id = $1 THEN addressee_id ELSE requester_id END AS fid
		FROM friendships
		WHERE (requester_id = $1 OR addressee_id = $1) AND status = 'accepted'
	)
	SELECT
		u.id,
		u.username,
		COALESCE(u.full_name, '') AS full_name,
		COALESCE(u.avatar_url, '') AS avatar_url,
		COUNT(mf.fid) AS mutual_count
	FROM users u
	LEFT JOIN my_friend_ids mf ON EXISTS (
		SELECT 1 FROM friendships f2
		WHERE f2.status = 'accepted'
		  AND (
			  (f2.requester_id = u.id AND f2.addressee_id = mf.fid) OR
			  (f2.addressee_id = u.id AND f2.requester_id = mf.fid)
		  )
	)
	WHERE u.id != $1
	  AND u.deactivated_at IS NULL
	  AND u.id NOT IN (SELECT uid FROM excluded_ids)
	GROUP BY u.id, u.username, u.full_name, u.avatar_url
	ORDER BY mutual_count DESC, RANDOM()
	LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list suggestions: %w", err)
	}
	defer rows.Close()

	out := make([]*friendship.Suggestion, 0)
	for rows.Next() {
		var s friendship.Suggestion
		if err := rows.Scan(&s.ID, &s.Username, &s.FullName, &s.AvatarURL, &s.MutualCount); err != nil {
			return nil, 0, fmt.Errorf("failed to scan suggestion row: %w", err)
		}
		out = append(out, &s)
	}

	// count total
	cntQ := `WITH excluded_ids AS (
		SELECT CASE WHEN requester_id = $1 THEN addressee_id ELSE requester_id END AS uid
		FROM friendships
		WHERE requester_id = $1 OR addressee_id = $1
	)
	SELECT COUNT(1) FROM users u WHERE u.id != $1 AND u.deactivated_at IS NULL AND u.id NOT IN (SELECT uid FROM excluded_ids)`

	var total int64
	if err := r.db.QueryRow(ctx, cntQ, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count suggestions: %w", err)
	}

	return out, total, nil
}
