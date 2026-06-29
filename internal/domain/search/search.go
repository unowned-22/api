package search

import "context"

// UserDocument is the shape of a user record stored in the search index.
// Only fields needed for mention/search UI are included — no sensitive data.
type UserDocument struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FullName  string `json:"full_name"`
	AvatarURL string `json:"avatar_url"`
}

// UserIndex defines the contract for user search index operations.
// Implementation lives in internal/infrastructure/search/.
type UserIndex interface {
	// Index adds or replaces a user document. Idempotent — safe to call on update.
	Index(ctx context.Context, doc UserDocument) error
	// Delete removes a user document by ID.
	Delete(ctx context.Context, userID int64) error
	// Search returns matching UserDocuments for the given query string.
	// Returns an empty slice (not nil) when nothing matches.
	Search(ctx context.Context, query string, limit int) ([]UserDocument, error)
}
