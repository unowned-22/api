package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

// PublishForCommunity  POST /api/v1/communities/{id}/stories
//
// Alias for Publish that forces author_type=community and community_id
// from the path, mirroring PostHandler.CreateForCommunity (Stage 3).
func (h *StoryHandler) PublishForCommunity(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	communityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid community id")
		return
	}

	var req dto.CreateStoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	// Community stories are always public — see AGENTS.md "Stories (Stage 6)".
	req.Visibility = "everyone"

	if err := validator.Validate(&req); err != nil {
		if ve, ok := err.(*validator.ValidationErrors); ok {
			response.SendValidationError(w, h.toFieldErrors(ve.Fields))
			return
		}
		response.SendBadRequest(w, "invalid request")
		return
	}

	slidesJSON, err := json.Marshal(req.Slides)
	if err != nil {
		response.SendBadRequest(w, "invalid slides payload")
		return
	}

	st, svcErr := h.storyService.Publish(r.Context(), userID, slidesJSON, req.Visibility, req.Duration, req.HiddenFrom, "community", &communityID)
	if svcErr != nil {
		response.SendError(w, r, svcErr)
		return
	}

	slides, _ := h.presignSlides(r.Context(), st.Slides)
	resp := dto.StoryResponse{
		ID:          st.ID,
		Visibility:  string(st.Visibility),
		Duration:    st.DurationHours,
		HiddenFrom:  st.HiddenFromUserIDs,
		Slides:      slides,
		CreatedAt:   st.CreatedAt.Format(time.RFC3339),
		ExpiresAt:   st.ExpiresAt.Format(time.RFC3339),
		AuthorType:  st.AuthorType,
		CommunityID: st.CommunityID,
	}
	response.SendSuccess(w, http.StatusCreated, resp)
}

// ListByCommunity  GET /api/v1/communities/{id}/stories
//
// Returns active stories published by the community. Caller must be a
// member of the community (any role) — enforced in StoryService.ListByCommunity.
func (h *StoryHandler) ListByCommunity(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	communityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid community id")
		return
	}
	limit, offset := getPaginationQueries(r)

	stories, svcErr := h.storyService.ListByCommunity(r.Context(), viewerID, communityID, limit, offset)
	if svcErr != nil {
		response.SendError(w, r, svcErr)
		return
	}

	out := make([]dto.StoryResponse, 0, len(stories))
	for _, st := range stories {
		slides, _ := h.presignSlides(r.Context(), st.Slides)
		out = append(out, dto.StoryResponse{
			ID:          st.ID,
			UserID:      st.UserID,
			Visibility:  string(st.Visibility),
			Duration:    st.DurationHours,
			HiddenFrom:  st.HiddenFromUserIDs,
			Slides:      slides,
			CreatedAt:   st.CreatedAt.Format(time.RFC3339),
			ExpiresAt:   st.ExpiresAt.Format(time.RFC3339),
			AuthorType:  st.AuthorType,
			CommunityID: st.CommunityID,
		})
	}
	response.SendSuccess(w, http.StatusOK, out)
}
