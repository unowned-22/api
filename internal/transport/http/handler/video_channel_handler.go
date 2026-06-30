// Package handler provides HTTP handlers for the /api/v1/channels routes.
//
// After Stage 2 migration, video_channels no longer exists as a separate
// table. These handlers are thin aliases over CommunityService, filtering
// to communities with type=video. They exist solely for mobile-client
// backward-compatibility; new code should use /api/v1/communities directly.
//
// Deprecation notice: all /api/v1/channels endpoints are deprecated and will
// be removed once all clients have migrated to /api/v1/communities.
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/community"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// VideoChannelHandler maps old /api/v1/channels routes to CommunityService.
type VideoChannelHandler struct {
	svc community.Service
}

func NewVideoChannelHandler(svc community.Service) *VideoChannelHandler {
	return &VideoChannelHandler{svc: svc}
}

func communityToChannelResponse(c *community.Community) *dto.ChannelResponse {
	return &dto.ChannelResponse{
		ID:               c.ID,
		UserID:           c.OwnerID,
		Name:             c.Name,
		Description:      c.Description,
		AvatarURL:        c.AvatarKey,
		BannerURL:        c.BannerKey,
		SubscribersCount: c.SubscribersCount,
		VideosCount:      c.VideosCount,
		CreatedAt:        c.CreatedAt,
	}
}

// CreateChannel  POST /api/v1/channels
// Alias: creates a community with type=video, slug derived from name.
func (h *VideoChannelHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	var req dto.CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	slug := slugify(req.Name) + "-" + strconv.FormatInt(userID, 10)
	c, err := h.svc.Create(r.Context(), userID, community.CreateRequest{
		Type:        community.TypeVideo,
		Visibility:  community.VisibilityPublic,
		Name:        req.Name,
		Slug:        slug,
		Description: req.Description,
	})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusCreated, communityToChannelResponse(c))
}

// GetChannel  GET /api/v1/channels/{id}
func (h *VideoChannelHandler) GetChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	c, svcErr := h.svc.GetByID(r.Context(), id)
	if svcErr != nil {
		response.SendError(w, r, svcErr)
		return
	}
	response.SendSuccess(w, http.StatusOK, communityToChannelResponse(c))
}

// GetMyChannel  GET /api/v1/channels/me
// Returns the caller's first owned video-type community.
func (h *VideoChannelHandler) GetMyChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	communities, err := h.svc.ListManageable(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	for _, c := range communities {
		if c.Type == community.TypeVideo && c.OwnerID == userID {
			response.SendSuccess(w, http.StatusOK, communityToChannelResponse(c))
			return
		}
	}
	response.SendNotFound(w, "channel not found")
}

// UpdateChannel  PATCH /api/v1/channels/{id}
func (h *VideoChannelHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req dto.UpdateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	c, svcErr := h.svc.Update(r.Context(), id, userID, community.UpdateRequest{
		Name:        req.Name,
		Description: req.Description,
		AvatarKey:   req.AvatarKey,
		BannerKey:   req.BannerKey,
	})
	if svcErr != nil {
		response.SendError(w, r, svcErr)
		return
	}
	response.SendSuccess(w, http.StatusOK, communityToChannelResponse(c))
}

// DeleteChannel  DELETE /api/v1/channels/{id}
func (h *VideoChannelHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.SoftDelete(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendNoContent(w)
}

// slugify converts a string to a URL-safe slug (simple, for alias use only).
func slugify(s string) string {
	out := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		switch {
		case c >= 'a' && c <= 'z':
			out = append(out, c)
		case c >= 'A' && c <= 'Z':
			out = append(out, c+32) // toLower
		case c >= '0' && c <= '9':
			out = append(out, c)
		default:
			if len(out) > 0 && out[len(out)-1] != '-' {
				out = append(out, '-')
			}
		}
	}
	// trim trailing dash
	for len(out) > 0 && out[len(out)-1] == '-' {
		out = out[:len(out)-1]
	}
	return string(out)
}
