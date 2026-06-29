package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/unowned-22/api/internal/domain/event"
	domainsearch "github.com/unowned-22/api/internal/domain/search"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/logger"
)

// userSearchIndexAction distinguishes whether the handler should upsert or
// remove a document from the search index for the given event.
type userSearchIndexAction int

const (
	userSearchIndexActionUpsert userSearchIndexAction = iota
	userSearchIndexActionDelete
)

// UserSearchIndexHandler keeps domain/search.UserIndex in sync with user data.
//
// It is constructed once per (event, action) pair, mirroring the
// AuditHandler pattern, and is meant to be combined with other handlers via
// MultiHandler when the event already has an unrelated subscriber:
//
//	domainevent.UserEmailVerified: handler.NewMultiHandler(
//	    domainevent.UserEmailVerified,
//	    handler.NewEmailVerifiedHandler(...),                 // existing provisioning
//	    handler.NewUserSearchIndexHandler(repo, idx, domainevent.UserEmailVerified, true),
//	)
//
// Only verified users are ever indexed: nothing publishes to the index
// outside of UserEmailVerified (first index) and UserProfileUpdated
// (re-index), and AccountDeactivated removes the document — there is no
// path that indexes an unverified user.
type UserSearchIndexHandler struct {
	repo      user.UserRepository
	index     domainsearch.UserIndex
	eventName event.Name
	action    userSearchIndexAction
}

func newUserSearchIndexHandler(repo user.UserRepository, index domainsearch.UserIndex, evt event.Name, action userSearchIndexAction) *UserSearchIndexHandler {
	return &UserSearchIndexHandler{repo: repo, index: index, eventName: evt, action: action}
}

// NewUserSearchIndexHandler builds an upsert handler: on Handle it loads the
// current user row and (re)indexes it. Use for UserEmailVerified and
// UserProfileUpdated.
func NewUserSearchIndexHandler(repo user.UserRepository, index domainsearch.UserIndex, evt event.Name) *UserSearchIndexHandler {
	return newUserSearchIndexHandler(repo, index, evt, userSearchIndexActionUpsert)
}

// NewUserSearchDeindexHandler builds a delete handler: on Handle it removes
// the user document from the index. Use for AccountDeactivated (and any
// future hard-delete event).
func NewUserSearchDeindexHandler(index domainsearch.UserIndex, evt event.Name) *UserSearchIndexHandler {
	return newUserSearchIndexHandler(nil, index, evt, userSearchIndexActionDelete)
}

func (h *UserSearchIndexHandler) EventName() event.Name {
	return h.eventName
}

func (h *UserSearchIndexHandler) Handle(ctx context.Context, payload []byte) error {
	if h.index == nil {
		// Meilisearch not configured for this environment — no-op.
		return nil
	}

	var p struct {
		UserID int64 `json:"user_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("user_search_index_handler: failed to unmarshal payload: %w", err)
	}
	if p.UserID == 0 {
		return fmt.Errorf("user_search_index_handler: user_id is required in event payload")
	}

	switch h.action {
	case userSearchIndexActionDelete:
		if err := h.index.Delete(ctx, p.UserID); err != nil {
			return fmt.Errorf("user_search_index_handler: failed to delete user %d: %w", p.UserID, err)
		}
		logger.Log.WithFields(map[string]interface{}{"user_id": p.UserID}).Info("user_search_index_handler: removed user from search index")
		return nil

	default: // upsert
		u, err := h.repo.GetByID(ctx, p.UserID)
		if err != nil {
			return fmt.Errorf("user_search_index_handler: failed to load user %d: %w", p.UserID, err)
		}
		// Defensive: never index a user whose email isn't verified, even if
		// this handler is ever wired to an event that shouldn't apply here.
		if u.EmailVerifiedAt == nil {
			logger.Log.WithFields(map[string]interface{}{"user_id": p.UserID}).Warn("user_search_index_handler: skipping index for unverified user")
			return nil
		}
		doc := domainsearch.UserDocument{
			ID:        u.ID,
			Username:  u.Username,
			FullName:  u.FullName,
			AvatarURL: u.AvatarURL,
		}
		if err := h.index.Index(ctx, doc); err != nil {
			return fmt.Errorf("user_search_index_handler: failed to index user %d: %w", p.UserID, err)
		}
		logger.Log.WithFields(map[string]interface{}{"user_id": p.UserID}).Info("user_search_index_handler: indexed user")
		return nil
	}
}

var _ event.Handler = (*UserSearchIndexHandler)(nil)
