package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/story"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type StoryHandler struct {
	storyService story.StoryService
}

// Note: handler uses domain/story service directly.

func NewStoryHandler(storyService story.StoryService) *StoryHandler {
	return &StoryHandler{storyService: storyService}
}

// Publish handles POST /stories
func (h *StoryHandler) Publish(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	var req dto.CreateStoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

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

	st, err := h.storyService.Publish(r.Context(), userID, slidesJSON, req.Visibility, req.Duration, req.HiddenFrom)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	var slides []json.RawMessage
	_ = json.Unmarshal(st.Slides, &slides)
	resp := dto.StoryResponse{
		ID:         st.ID,
		Visibility: string(st.Visibility),
		Duration:   st.DurationHours,
		HiddenFrom: st.HiddenFromUserIDs,
		Slides:     slides,
		CreatedAt:  st.CreatedAt.Format(time.RFC3339),
		ExpiresAt:  st.ExpiresAt.Format(time.RFC3339),
	}
	response.SendSuccess(w, http.StatusCreated, resp)
}

func (h *StoryHandler) toFieldErrors(fields []validator.FieldError) []response.ValidationFieldError {
	out := make([]response.ValidationFieldError, 0, len(fields))
	for _, f := range fields {
		out = append(out, response.ValidationFieldError{Field: f.Field, Message: f.Message})
	}
	return out
}

// ListMine handles GET /stories/me
func (h *StoryHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	sts, err := h.storyService.ListMyStories(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.StoryResponse, 0, len(sts))
	for _, s := range sts {
		var slides []json.RawMessage
		_ = json.Unmarshal(s.Slides, &slides)
		out = append(out, dto.StoryResponse{
			ID:         s.ID,
			Visibility: string(s.Visibility),
			Duration:   s.DurationHours,
			HiddenFrom: s.HiddenFromUserIDs,
			Slides:     slides,
			CreatedAt:  s.CreatedAt.Format(time.RFC3339),
			ExpiresAt:  s.ExpiresAt.Format(time.RFC3339),
		})
	}
	response.SendSuccess(w, http.StatusOK, out)
}

// Delete handles DELETE /stories/{id}
func (h *StoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}

	if err := h.storyService.Delete(r.Context(), userID, id); err != nil {
		response.SendError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
