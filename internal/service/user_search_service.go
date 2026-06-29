package service

import (
	"context"
	"strings"

	domainsearch "github.com/unowned-22/api/internal/domain/search"
)

// maxUserSearchLimit caps the page size regardless of what the client asks
// for. There is no privacy filtering yet (see AGENTS.md), so this is the
// only guard against pulling the whole index in one request.
const maxUserSearchLimit = 20

// UserSearchService exposes read-only search over the user index
// (internal/domain/search.UserIndex), independent of UserService — it has
// nothing to do with profile/storage management.
type UserSearchService struct {
	index domainsearch.UserIndex
}

func NewUserSearchService(index domainsearch.UserIndex) *UserSearchService {
	return &UserSearchService{index: index}
}

// Search returns up to limit matching users for query. Returns an empty
// (non-nil) slice if the index isn't configured or nothing matches.
func (s *UserSearchService) Search(ctx context.Context, query string, limit int) ([]domainsearch.UserDocument, error) {
	query = strings.TrimSpace(query)
	if query == "" || s.index == nil {
		return []domainsearch.UserDocument{}, nil
	}
	if limit <= 0 || limit > maxUserSearchLimit {
		limit = maxUserSearchLimit
	}
	return s.index.Search(ctx, query, limit)
}
