package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/feed"
)

// FeedRepository reads from the feed_items SQL view (UNION ALL of
// user_posts + community_posts, see migration 000006).
type FeedRepository struct{ pool *pgxpool.Pool }

func NewFeedRepository(pool *pgxpool.Pool) *FeedRepository {
	return &FeedRepository{pool: pool}
}

// ListHomeFeed returns the personalised feed for userID:
//   - the user's own posts and accepted-friends' posts (source_type='user')
//   - posts from communities the user is a member of (source_type='community'),
//     optionally restricted to communities of a given type.
//
// Friend IDs come from `friendships` (status='accepted', either direction);
// community IDs come from `community_members`. Both are resolved inline via
// subqueries rather than a separate round-trip, to keep this a single query.
func (r *FeedRepository) ListHomeFeed(ctx context.Context, userID int64, typeFilter *string, limit, offset int) ([]*feed.Item, error) {
	var (
		q    string
		args []any
	)
	if typeFilter != nil {
		q = `
			SELECT source_type, id, owner_id, community_id, text, media,
			       likes_count, comments_count, created_at
			FROM feed_items
			WHERE
			    (source_type = 'user' AND owner_id IN (
			        SELECT $1::bigint
			        UNION
			        SELECT CASE WHEN f.requester_id = $1 THEN f.addressee_id ELSE f.requester_id END
			        FROM friendships f
			        WHERE (f.requester_id = $1 OR f.addressee_id = $1) AND f.status = 'accepted'
			    ))
			 OR (source_type = 'community' AND community_id IN (
			        SELECT cm.community_id
			        FROM community_members cm
			        INNER JOIN communities c ON c.id = cm.community_id
			        WHERE cm.user_id = $1 AND c.type = $2
			    ))
			ORDER BY created_at DESC
			LIMIT $3 OFFSET $4`
		args = []any{userID, *typeFilter, limit, offset}
	} else {
		q = `
			SELECT source_type, id, owner_id, community_id, text, media,
			       likes_count, comments_count, created_at
			FROM feed_items
			WHERE
			    (source_type = 'user' AND owner_id IN (
			        SELECT $1::bigint
			        UNION
			        SELECT CASE WHEN f.requester_id = $1 THEN f.addressee_id ELSE f.requester_id END
			        FROM friendships f
			        WHERE (f.requester_id = $1 OR f.addressee_id = $1) AND f.status = 'accepted'
			    ))
			 OR (source_type = 'community' AND community_id IN (
			        SELECT community_id FROM community_members WHERE user_id = $1
			    ))
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3`
		args = []any{userID, limit, offset}
	}

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*feed.Item
	for rows.Next() {
		it := &feed.Item{}
		if err := rows.Scan(
			&it.SourceType, &it.ID, &it.OwnerID, &it.CommunityID,
			&it.Text, &it.Media, &it.LikesCount, &it.CommentsCount, &it.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

var _ feed.Repository = (*FeedRepository)(nil)
