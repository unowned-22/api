package service

import (
	"context"

	"github.com/unowned-22/api/internal/domain/feed"
)

// FeedService is a thin wrapper over feed.Repository — all the real
// personalisation logic (friends + subscribed communities) lives in the
// single feed_items SQL query (see FeedRepository.ListHomeFeed) rather
// than being assembled in Go, to avoid N+1 queries.
type FeedService struct {
	repo feed.Repository
}

func NewFeedService(repo feed.Repository) *FeedService {
	return &FeedService{repo: repo}
}

// ListHomeFeed returns the personalised home feed for userID.
// typeFilter, if non-nil, restricts community posts to that community type
// (e.g. "video"); user posts are always included regardless of typeFilter.
func (s *FeedService) ListHomeFeed(ctx context.Context, userID int64, typeFilter *string, limit, offset int) ([]*feed.Item, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.ListHomeFeed(ctx, userID, typeFilter, limit, offset)
}
