package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/community"
	"github.com/unowned-22/api/internal/errs"
)

// CommunityRepository implements community.Repository using raw pgx SQL.
type CommunityRepository struct{ pool *pgxpool.Pool }

func NewCommunityRepository(pool *pgxpool.Pool) *CommunityRepository {
	return &CommunityRepository{pool: pool}
}

// ── helpers ─────────────────────────────────────────────────────────────────

const communitySelectCols = `
	id, owner_id, type, visibility, name, slug, description,
	avatar_key, banner_key,
	members_count, posts_count, subscribers_count, videos_count,
	created_at, updated_at, deleted_at`

func scanCommunity(row pgx.Row) (*community.Community, error) {
	c := &community.Community{}
	err := row.Scan(
		&c.ID, &c.OwnerID, &c.Type, &c.Visibility, &c.Name, &c.Slug, &c.Description,
		&c.AvatarKey, &c.BannerKey,
		&c.MembersCount, &c.PostsCount, &c.SubscribersCount, &c.VideosCount,
		&c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrCommunityNotFound
	}
	return c, err
}

// ── community CRUD ───────────────────────────────────────────────────────────

func (r *CommunityRepository) Create(ctx context.Context, c *community.Community) error {
	q := `
		INSERT INTO communities
			(owner_id, type, visibility, name, slug, description, avatar_key, banner_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, created_at, updated_at`
	err := r.pool.QueryRow(ctx, q,
		c.OwnerID, string(c.Type), string(c.Visibility),
		c.Name, c.Slug, c.Description, c.AvatarKey, c.BannerKey,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if isPgUniqueViolation(err) {
		return errs.ErrCommunitySlugTaken
	}
	return err
}

func (r *CommunityRepository) GetByID(ctx context.Context, id int64) (*community.Community, error) {
	q := `SELECT ` + communitySelectCols + ` FROM communities WHERE id=$1 AND deleted_at IS NULL`
	return scanCommunity(r.pool.QueryRow(ctx, q, id))
}

func (r *CommunityRepository) GetBySlug(ctx context.Context, slug string) (*community.Community, error) {
	q := `SELECT ` + communitySelectCols + ` FROM communities WHERE slug=$1 AND deleted_at IS NULL`
	return scanCommunity(r.pool.QueryRow(ctx, q, slug))
}

func (r *CommunityRepository) Update(ctx context.Context, c *community.Community) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE communities
		   SET type=$1, visibility=$2, name=$3, description=$4,
		       avatar_key=$5, banner_key=$6, updated_at=NOW()
		 WHERE id=$7`,
		string(c.Type), string(c.Visibility), c.Name, c.Description,
		c.AvatarKey, c.BannerKey, c.ID,
	)
	return err
}

func (r *CommunityRepository) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE communities SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1`, id)
	return err
}

// ── listings / search ────────────────────────────────────────────────────────

func (r *CommunityRepository) ListByOwner(ctx context.Context, ownerID int64) ([]*community.Community, error) {
	q := `SELECT ` + communitySelectCols + `
		  FROM communities WHERE owner_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC`
	return r.queryCommunities(ctx, q, ownerID)
}

func (r *CommunityRepository) ListByType(ctx context.Context, t community.Type, limit, offset int) ([]*community.Community, error) {
	q := `SELECT ` + communitySelectCols + `
		  FROM communities WHERE type=$1 AND deleted_at IS NULL
		  ORDER BY members_count DESC LIMIT $2 OFFSET $3`
	return r.queryCommunities(ctx, q, string(t), limit, offset)
}

func (r *CommunityRepository) Search(ctx context.Context, q string, t *community.Type, limit, offset int) ([]*community.Community, error) {
	if t != nil {
		sql := `SELECT ` + communitySelectCols + `
			FROM communities
			WHERE deleted_at IS NULL AND type=$1
			  AND (name ILIKE '%' || $2 || '%' OR slug ILIKE '%' || $2 || '%')
			ORDER BY members_count DESC LIMIT $3 OFFSET $4`
		return r.queryCommunities(ctx, sql, string(*t), q, limit, offset)
	}
	sql := `SELECT ` + communitySelectCols + `
		FROM communities
		WHERE deleted_at IS NULL
		  AND (name ILIKE '%' || $1 || '%' OR slug ILIKE '%' || $1 || '%')
		ORDER BY members_count DESC LIMIT $2 OFFSET $3`
	return r.queryCommunities(ctx, sql, q, limit, offset)
}

func (r *CommunityRepository) queryCommunities(ctx context.Context, q string, args ...any) ([]*community.Community, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*community.Community
	for rows.Next() {
		c, err := scanCommunity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ── members ──────────────────────────────────────────────────────────────────

func (r *CommunityRepository) AddMember(ctx context.Context, m *community.Member) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO community_members (community_id, user_id, role, joined_at)
		VALUES ($1,$2,$3,NOW())
		ON CONFLICT DO NOTHING`,
		m.CommunityID, m.UserID, string(m.Role),
	)
	return err
}

func (r *CommunityRepository) RemoveMember(ctx context.Context, communityID, userID int64) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM community_members WHERE community_id=$1 AND user_id=$2`,
		communityID, userID,
	)
	return err
}

func (r *CommunityRepository) UpdateMemberRole(ctx context.Context, communityID, userID int64, role community.MemberRole) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE community_members SET role=$1 WHERE community_id=$2 AND user_id=$3`,
		string(role), communityID, userID,
	)
	return err
}

func (r *CommunityRepository) GetMember(ctx context.Context, communityID, userID int64) (*community.Member, error) {
	m := &community.Member{}
	err := r.pool.QueryRow(ctx,
		`SELECT community_id, user_id, role, joined_at FROM community_members WHERE community_id=$1 AND user_id=$2`,
		communityID, userID,
	).Scan(&m.CommunityID, &m.UserID, &m.Role, &m.JoinedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrNotCommunityMember
	}
	return m, err
}

func (r *CommunityRepository) ListMembers(ctx context.Context, communityID int64, roleFilter *community.MemberRole, limit, offset int) ([]*community.Member, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if roleFilter != nil {
		rows, err = r.pool.Query(ctx,
			`SELECT community_id, user_id, role, joined_at FROM community_members
			 WHERE community_id=$1 AND role=$2 ORDER BY joined_at LIMIT $3 OFFSET $4`,
			communityID, string(*roleFilter), limit, offset)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT community_id, user_id, role, joined_at FROM community_members
			 WHERE community_id=$1 ORDER BY joined_at LIMIT $2 OFFSET $3`,
			communityID, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*community.Member
	for rows.Next() {
		m := &community.Member{}
		if err := rows.Scan(&m.CommunityID, &m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *CommunityRepository) ListMemberCommunityIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT community_id FROM community_members WHERE user_id=$1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *CommunityRepository) IsMember(ctx context.Context, communityID, userID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM community_members WHERE community_id=$1 AND user_id=$2)`,
		communityID, userID,
	).Scan(&exists)
	return exists, err
}

func (r *CommunityRepository) IsAdminOrOwner(ctx context.Context, communityID, userID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM community_members WHERE community_id=$1 AND user_id=$2 AND role IN ('owner','admin'))`,
		communityID, userID,
	).Scan(&exists)
	return exists, err
}

// ── counters ─────────────────────────────────────────────────────────────────

func (r *CommunityRepository) IncrMembersCount(ctx context.Context, communityID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE communities SET members_count=members_count+1 WHERE id=$1`, communityID)
	return err
}

func (r *CommunityRepository) DecrMembersCount(ctx context.Context, communityID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE communities SET members_count=GREATEST(members_count-1,0) WHERE id=$1`, communityID)
	return err
}

func (r *CommunityRepository) IncrPostsCount(ctx context.Context, communityID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE communities SET posts_count=posts_count+1 WHERE id=$1`, communityID)
	return err
}

func (r *CommunityRepository) DecrPostsCount(ctx context.Context, communityID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE communities SET posts_count=GREATEST(posts_count-1,0) WHERE id=$1`, communityID)
	return err
}

func (r *CommunityRepository) IncrVideosCount(ctx context.Context, communityID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE communities SET videos_count=videos_count+1 WHERE id=$1`, communityID)
	return err
}

func (r *CommunityRepository) DecrVideosCount(ctx context.Context, communityID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE communities SET videos_count=GREATEST(videos_count-1,0) WHERE id=$1`, communityID)
	return err
}

// ── compile-time interface check ─────────────────────────────────────────────

var _ community.Repository = (*CommunityRepository)(nil)

// ── internal helpers ─────────────────────────────────────────────────────────

// isPgUniqueViolation checks for Postgres error code 23505.
// Re-defined locally to avoid adding a shared util just for this; if the
// project already has such a helper, replace these calls with it.
func isPgUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// pgconn.PgError carries Code
	type pgErr interface{ SQLState() string }
	var pe pgErr
	if errors.As(err, &pe) {
		return pe.SQLState() == "23505"
	}
	return false
}
