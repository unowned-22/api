package handler

import (
	"context"
	"encoding/json"
	"fmt"
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
		var authorName, authorUsername, authorAvatar string
		if u, err := h.userService.GetProfile(r.Context(), s.UserID); err == nil && u != nil {
			authorName = u.FullName
			authorUsername = u.Username
			authorAvatar = u.AvatarURL
		}
		out = append(out, dto.StoryResponse{
			ID:             s.ID,
			UserID:         s.UserID,
			AuthorName:     authorName,
			AuthorUsername: authorUsername,
			AuthorAvatar:   authorAvatar,
			Visibility:     string(s.Visibility),
			Duration:       s.DurationHours,
			HiddenFrom:     s.HiddenFromUserIDs,
			Slides:         slides,
			CreatedAt:      s.CreatedAt.Format(time.RFC3339),
			ExpiresAt:      s.ExpiresAt.Format(time.RFC3339),
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
		// Build stripped feed slides: only id, rendered_url (presigned) and seen flag.
		var rawSlides []map[string]any
		if err := json.Unmarshal(s.Slides, &rawSlides); err != nil {
			rawSlides = nil
		}

		// helper to determine seen status for a slide index
		isSeen := func(m map[int]bool, idx int) bool {
			if m == nil {
				return false
			}
			if v, ok := m[-1]; ok && v {
				return true
			}
			if v, ok := m[idx]; ok && v {
				return true
			}
			return false
		}

		feedSlides := make([]json.RawMessage, 0, len(rawSlides))
		storyViews := views[s.ID]
		for i, rs := range rawSlides {
			stripped := dto.FeedSlideResponse{ID: fmt.Sprintf("%v", rs["id"]), Seen: isSeen(storyViews, i)}
			// presign rendered_url if present
			if rv, ok := rs["rendered_url"].(string); ok && rv != "" {
				if strings.Contains(rv, "/") {
					if stg, ok := h.storage.(domainstorage.Storage); ok {
						if presigned, err := stg.PresignURL(r.Context(), h.storageBucket(), rv, 15*time.Minute); err == nil {
							stripped.RenderedURL = presigned
						}
					} else if o, ok := h.storage.(domainstorage.ObjectStorage); ok {
						if presigned, err := o.GetURL(r.Context(), h.storageBucket(), rv, 15*time.Minute); err == nil {
							stripped.RenderedURL = presigned
						}
					}
				}
			}
			b, _ := json.Marshal(stripped)
			feedSlides = append(feedSlides, b)
		}

		// author metadata (best-effort)
		var authorName, authorUsername, authorAvatar string
		if u, err := h.userService.GetProfile(r.Context(), s.UserID); err == nil && u != nil {
			authorName = u.FullName
			authorUsername = u.Username
			authorAvatar = u.AvatarURL
		}

		out = append(out, dto.StoryResponse{
			ID:             s.ID,
			UserID:         s.UserID,
			AuthorName:     authorName,
			AuthorUsername: authorUsername,
			AuthorAvatar:   authorAvatar,
			Visibility:     string(s.Visibility),
			Duration:       s.DurationHours,
			HiddenFrom:     s.HiddenFromUserIDs,
			Slides:         feedSlides,
			CreatedAt:      s.CreatedAt.Format(time.RFC3339),
			ExpiresAt:      s.ExpiresAt.Format(time.RFC3339),
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
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if payload.Message == "" {
		response.SendBadRequest(w, "message is required")
		return
	}
	if err := h.storyService.Reply(r.Context(), userID, id, payload.Message); err != nil {
		response.SendError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// presignSlides walks the slides JSON array and replaces any media background
// `url` fields that contain storage keys with short-lived presigned GET URLs.
// It also presigns an optional top-level `rendered_url` key per slide.
func (h *StoryHandler) presignSlides(ctx context.Context, slidesJSON []byte) ([]json.RawMessage, error) {
	var slidesArr []map[string]any
	if err := json.Unmarshal(slidesJSON, &slidesArr); err != nil {
		return nil, err
	}
	for i := range slidesArr {
		// look for background.media.url
		bg, ok := slidesArr[i]["background"].(map[string]any)
		if ok && bg != nil {
			if kind, _ := bg["kind"].(string); kind == "media" {
				if urlv, ok := bg["url"].(string); ok && urlv != "" {
					if strings.Contains(urlv, "/") {
						if s, ok := h.storage.(domainstorage.Storage); ok {
							presigned, err := s.PresignURL(ctx, h.storageBucket(), urlv, 15*time.Minute)
							if err == nil {
								bg["url"] = presigned
							}
						} else if o, ok := h.storage.(domainstorage.ObjectStorage); ok {
							presigned, err := o.GetURL(ctx, h.storageBucket(), urlv, 15*time.Minute)
							if err == nil {
								bg["url"] = presigned
							}
						}
					}
				}
			}
		}

		// also presign top-level rendered_url if present (frontend may upload a composite image)
		if rv, ok := slidesArr[i]["rendered_url"].(string); ok && rv != "" {
			if strings.Contains(rv, "/") {
				if s, ok := h.storage.(domainstorage.Storage); ok {
					presigned, err := s.PresignURL(ctx, h.storageBucket(), rv, 15*time.Minute)
					if err == nil {
						slidesArr[i]["rendered_url"] = presigned
					}
				} else if o, ok := h.storage.(domainstorage.ObjectStorage); ok {
					presigned, err := o.GetURL(ctx, h.storageBucket(), rv, 15*time.Minute)
					if err == nil {
						slidesArr[i]["rendered_url"] = presigned
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
