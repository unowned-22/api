package feed

import (
	"context"
	"time"
)

// SourceType identifies which table a FeedItem came from.
type SourceType string

const (
	SourceUser      SourceType = "user"
	SourceCommunity SourceType = "community"
)

// Item is a single row read from the feed_items SQL view (UNION ALL of
// user_posts + community_posts). Media is kept as raw JSON bytes — the
// feed layer doesn't need to parse it, only pass it through to the
// transport layer for serialisation, mirroring the story-slides pattern
// (see AGENTS.md "Stories feature notes").
type Item struct {
	SourceType    SourceType
	ID            int64
	OwnerID       int64  // user_posts.user_id or community_posts.author_user_id
	CommunityID   *int64 // nil for SourceUser
	Text          string
	Media         []byte // raw JSONB
	LikesCount    int64
	CommentsCount int64
	CreatedAt     time.Time
}

// Repository reads from the feed_items view.
type Repository interface {
	// ListHomeFeed returns the personalised home feed for userID:
	// the user's own + friends' posts, plus posts from communities the
	// user is a member of. typeFilter, if non-nil, restricts community
	// posts to communities of that type (e.g. "video"); user posts are
	// always included regardless of typeFilter.
	ListHomeFeed(ctx context.Context, userID int64, typeFilter *string, limit, offset int) ([]*Item, error)
}
