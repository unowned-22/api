package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	domainstorage "github.com/unowned-22/api/internal/domain/storage"
	"github.com/unowned-22/api/internal/domain/story"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type StoryHandler struct {
	storyService story.StoryService
	storage      domainstorage.Storage
	publicBucket string
	userService  user.UserService
}

// Note: handler uses domain/story service directly.

func NewStoryHandler(storyService story.StoryService, storage domainstorage.Storage, publicBucket string, userService user.UserService) *StoryHandler {
	return &StoryHandler{storyService: storyService, storage: storage, publicBucket: publicBucket, userService: userService}
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

	slides, _ := h.presignSlides(r.Context(), st.Slides)
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
		slides, _ := h.presignSlides(r.Context(), s.Slides)
		// author metadata
		var authorName, authorAvatar string
		if u, err := h.userService.GetProfile(r.Context(), s.UserID); err == nil && u != nil {
			authorName = u.FullName
			authorAvatar = u.AvatarURL
		}
		out = append(out, dto.StoryResponse{
			ID:           s.ID,
			UserID:       s.UserID,
			AuthorName:   authorName,
			AuthorAvatar: authorAvatar,
			Visibility:   string(s.Visibility),
			Duration:     s.DurationHours,
			HiddenFrom:   s.HiddenFromUserIDs,
			Slides:       slides,
			CreatedAt:    s.CreatedAt.Format(time.RFC3339),
			ExpiresAt:    s.ExpiresAt.Format(time.RFC3339),
		})
	}
	response.SendSuccess(w, http.StatusOK, out)
}

// Feed handles GET /stories/feed
func (h *StoryHandler) Feed(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	sts, err := h.storyService.Feed(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	// fetch viewer's existing views so we can mark per-slide seen flags
	views, _ := h.storyService.ListViewsByViewer(r.Context(), userID)

	out := make([]dto.StoryResponse, 0, len(sts))
	for _, s := range sts {
		// presign slides and unmarshal to annotate seen flags
		var slidesArr []map[string]any
		if err := json.Unmarshal(s.Slides, &slidesArr); err != nil {
			slidesArr = nil
		}
		// presign URLs in-place
		for i := range slidesArr {
			// for each slide, presign media URLs (reuse presignSlides helper by marshalling single slide)
			b, _ := json.Marshal(slidesArr[i])
			pres, _ := h.presignSlides(r.Context(), b)
			if len(pres) > 0 {
				var updated map[string]any
				_ = json.Unmarshal(pres[0], &updated)
				slidesArr[i] = updated
			}
			// annotate seen status from views map
			if m, ok := views[s.ID]; ok {
				if _, seenAll := m[-1]; seenAll {
					slidesArr[i]["seen"] = true
				} else if _, seenSlide := m[i]; seenSlide {
					slidesArr[i]["seen"] = true
				}
			}
		}
		// marshal back to json.RawMessage slice
		slides := make([]json.RawMessage, 0, len(slidesArr))
		for _, si := range slidesArr {
			b, _ := json.Marshal(si)
			slides = append(slides, b)
		}

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

// View handles POST /stories/{id}/view
func (h *StoryHandler) View(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := contextx.UserID(r.Context())
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
	// optional JSON body { "slide_index": number }
	var payload struct {
		SlideIndex *int `json:"slide_index"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	if err := h.storyService.AddView(r.Context(), viewerID, id, payload.SlideIndex); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusNoContent, nil)
}

// Like handles POST /stories/{id}/like
func (h *StoryHandler) Like(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	if err := h.storyService.Like(r.Context(), userID, id); err != nil {
		response.SendError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Unlike handles POST /stories/{id}/unlike
func (h *StoryHandler) Unlike(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	if err := h.storyService.Unlike(r.Context(), userID, id); err != nil {
		response.SendError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reply handles POST /stories/{id}/reply
func (h *StoryHandler) Reply(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	var payload struct {
		message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if payload.message == "" {
		response.SendBadRequest(w, "message is required")
		return
	}
	if err := h.storyService.Reply(r.Context(), userID, id, payload.message); err != nil {
		response.SendError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// presignSlides walks the slides JSON array and replaces any media background
// `url` fields that contain storage keys with short-lived presigned GET URLs.
func (h *StoryHandler) presignSlides(ctx context.Context, slidesJSON []byte) ([]json.RawMessage, error) {
	var slidesArr []map[string]any
	if err := json.Unmarshal(slidesJSON, &slidesArr); err != nil {
		return nil, err
	}
	for i := range slidesArr {
		// look for background.media.url
		bg, ok := slidesArr[i]["background"].(map[string]any)
		if !ok || bg == nil {
			continue
		}
		if kind, _ := bg["kind"].(string); kind != "media" {
			continue
		}
		if urlv, ok := bg["url"].(string); ok && urlv != "" {
			// If url looks like a storage key (contains '/'), presign it.
			if strings.Contains(urlv, "/") {
				// Try storage.PresignURL via the Storage interface.
				if s, ok := h.storage.(domainstorage.Storage); ok {
					presigned, err := s.PresignURL(ctx, h.storageBucket(), urlv, 15*time.Minute)
					if err == nil {
						bg["url"] = presigned
					}
				} else {
					// fallback to ObjectStorage GetURL if necessary
					if o, ok := h.storage.(domainstorage.ObjectStorage); ok {
						presigned, err := o.GetURL(ctx, h.storageBucket(), urlv, 15*time.Minute)
						if err == nil {
							bg["url"] = presigned
						}
					}
				}
			}
		}
	}
	out := make([]json.RawMessage, 0, len(slidesArr))
	for _, s := range slidesArr {
		b, _ := json.Marshal(s)
		out = append(out, b)
	}
	return out, nil
}

func (h *StoryHandler) storageBucket() string {
	if strings.TrimSpace(h.publicBucket) != "" {
		return h.publicBucket
	}
	return ""
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
