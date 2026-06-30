package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/service"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// PostHandler exposes /api/v1/posts and the /api/v1/communities/{id}/posts alias.
type PostHandler struct {
	svc *service.PostService
}

func NewPostHandler(svc *service.PostService) *PostHandler {
	return &PostHandler{svc: svc}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func toMediaResponse(in []dto.MediaItemRequest) []dto.MediaItemResponse {
	out := make([]dto.MediaItemResponse, 0, len(in))
	for _, m := range in {
		out = append(out, dto.MediaItemResponse{
			Type: m.Type, StorageKey: m.StorageKey,
			Width: m.Width, Height: m.Height, DurationS: m.DurationS,
		})
	}
	return out
}

func (h *PostHandler) toCreateResponse(res *service.PostResult) *dto.CreatePostResponse {
	out := &dto.CreatePostResponse{SourceType: string(res.SourceType)}
	if res.UserPost != nil {
		p := res.UserPost
		media := make([]dto.MediaItemResponse, 0, len(p.Media))
		for _, m := range p.Media {
			media = append(media, dto.MediaItemResponse{
				Type: m.Type, StorageKey: m.StorageKey,
				Width: m.Width, Height: m.Height, DurationS: m.DurationS,
			})
		}
		out.UserPost = &dto.UserPostResponse{
			ID: p.ID, UserID: p.UserID, Text: p.Text, Media: media,
			Visibility: string(p.Visibility), LikesCount: p.LikesCount,
			CommentsCount: p.CommentsCount, CreatedAt: p.CreatedAt,
		}
	}
	if res.CommunityPost != nil {
		p := res.CommunityPost
		media := make([]dto.MediaItemResponse, 0, len(p.Media))
		for _, m := range p.Media {
			media = append(media, dto.MediaItemResponse{
				Type: m.Type, StorageKey: m.StorageKey,
				Width: m.Width, Height: m.Height, DurationS: m.DurationS,
			})
		}
		out.CommunityPost = &dto.CommunityPostResponse{
			ID: p.ID, CommunityID: p.CommunityID, AuthorUserID: p.AuthorUserID,
			Text: p.Text, Media: media, VideoID: p.VideoID, Pinned: p.Pinned,
			LikesCount: p.LikesCount, CommentsCount: p.CommentsCount, CreatedAt: p.CreatedAt,
		}
	}
	return out
}

func (h *PostHandler) toMediaInput(in []dto.MediaItemRequest) []service.MediaItemInput {
	out := make([]service.MediaItemInput, 0, len(in))
	for _, m := range in {
		out = append(out, service.MediaItemInput{
			Type: m.Type, StorageKey: m.StorageKey,
			Width: m.Width, Height: m.Height, DurationS: m.DurationS,
		})
	}
	return out
}

// ── POST /api/v1/posts ───────────────────────────────────────────────────────

// Create handles both user and community post creation, discriminated by
// req.AuthorType. POST /api/v1/posts
func (h *PostHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	var req dto.CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	res, err := h.svc.Create(r.Context(), userID, service.CreatePostRequest{
		AuthorType:  service.AuthorType(req.AuthorType),
		CommunityID: req.CommunityID,
		Text:        req.Text,
		Media:       h.toMediaInput(req.Media),
		Visibility:  req.Visibility,
	})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusCreated, h.toCreateResponse(res))
}

// CreateForCommunity is the alias for POST /api/v1/communities/{id}/posts —
// forces author_type=community and community_id={id} from the path.
func (h *PostHandler) CreateForCommunity(w http.ResponseWriter, r *http.Request) {
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
	var req dto.CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	res, svcErr := h.svc.Create(r.Context(), userID, service.CreatePostRequest{
		AuthorType:  service.AuthorTypeCommunity,
		CommunityID: &communityID,
		Text:        req.Text,
		Media:       h.toMediaInput(req.Media),
	})
	if svcErr != nil {
		response.SendError(w, r, svcErr)
		return
	}
	response.SendSuccess(w, http.StatusCreated, h.toCreateResponse(res))
}

// ── DELETE /api/v1/posts/{id}?source=user|community ─────────────────────────

// Delete soft-deletes a post. Requires ?source=user|community to know which
// table to look in (the two post tables have independent ID sequences).
func (h *PostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid post id")
		return
	}
	source := r.URL.Query().Get("source")
	if source != string(service.AuthorTypeUser) && source != string(service.AuthorTypeCommunity) {
		response.SendError(w, r, errs.ErrInvalidPostPayload)
		return
	}
	if svcErr := h.svc.Delete(r.Context(), id, service.AuthorType(source), userID); svcErr != nil {
		response.SendError(w, r, svcErr)
		return
	}
	response.SendNoContent(w)
}
