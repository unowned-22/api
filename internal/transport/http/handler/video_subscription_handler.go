// Package handler provides HTTP handlers for video subscription routes.
//
// After Stage 2, subscriptions are stored in community_members(role=subscriber).
// These handlers delegate to community.Service.Join / Leave and remain for
// mobile-client backward-compatibility.
//
// Deprecated: use POST /api/v1/communities/{id}/join and /leave instead.
package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/community"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// VideoSubscriptionHandler maps old subscribe/unsubscribe routes to
// community.Service.Join / Leave.
type VideoSubscriptionHandler struct {
	svc community.Service
}

func NewVideoSubscriptionHandler(svc community.Service) *VideoSubscriptionHandler {
	return &VideoSubscriptionHandler{svc: svc}
}

// Subscribe  POST /api/v1/channels/{id}/subscribe
// Alias: community.Service.Join (role becomes subscriber for public community).
func (h *VideoSubscriptionHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	channelID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.Join(r.Context(), channelID, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendNoContent(w)
}

// Unsubscribe  DELETE /api/v1/channels/{id}/subscribe
// Alias: community.Service.Leave.
func (h *VideoSubscriptionHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	channelID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.Leave(r.Context(), channelID, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendNoContent(w)
}

// GetSubscriberCount  GET /api/v1/channels/{id}/subscribers/count
// Reads subscribers_count directly from the community.
func (h *VideoSubscriptionHandler) GetSubscriberCount(w http.ResponseWriter, r *http.Request) {
	channelID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	c, svcErr := h.svc.GetByID(r.Context(), channelID)
	if svcErr != nil {
		response.SendError(w, r, svcErr)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]int64{"count": c.SubscribersCount})
}

// ListMySubscriptions  GET /api/v1/channels/subscriptions
// Returns the community IDs (all types, but callers expect only video ones).
func (h *VideoSubscriptionHandler) ListMySubscriptions(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	// ListManageable covers admin/owner only; for subscriber listing we need
	// a different approach. For now, return IDs from manageable + subscribed.
	// TODO: add Service.ListSubscribedCommunityIDs for pure subscriber view.
	manageable, err := h.svc.ListManageable(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	ids := make([]int64, 0, len(manageable))
	for _, c := range manageable {
		if c.Type == community.TypeVideo {
			ids = append(ids, c.ID)
		}
	}
	response.SendSuccess(w, http.StatusOK, ids)
}
