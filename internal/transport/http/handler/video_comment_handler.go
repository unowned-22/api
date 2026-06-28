package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/videocomment"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type VideoCommentHandler struct{ svc videocomment.Service }

func NewVideoCommentHandler(s videocomment.Service) *VideoCommentHandler {
	return &VideoCommentHandler{svc: s}
}

func (h *VideoCommentHandler) ListComments(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := contextx.UserID(r.Context())
	videoID, err := strconv.ParseInt(chi.URLParam(r, "videoID"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	limit, offset := getPaginationQueries(r)
	items, total, err := h.svc.ListComments(r.Context(), videoID, viewerID, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.VideoCommentListResponse{Comments: h.toResponses(items), Total: total})
}

func (h *VideoCommentHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	videoID, err := strconv.ParseInt(chi.URLParam(r, "videoID"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req dto.CreateVideoCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.SendValidationError(w, []response.ValidationFieldError{{Field: "body", Message: "invalid"}})
		return
	}
	c, err := h.svc.AddComment(r.Context(), videoID, userID, req.ParentID, req.Body)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusCreated, h.toResponse(c))
}

func (h *VideoCommentHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "commentID"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.DeleteComment(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusNoContent, nil)
}

func (h *VideoCommentHandler) ListReplies(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := contextx.UserID(r.Context())
	parentID, err := strconv.ParseInt(chi.URLParam(r, "commentID"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	items, err := h.svc.ListReplies(r.Context(), parentID, viewerID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.VideoCommentListResponse{Comments: h.toResponses(items), Total: len(items)})
}

func (h *VideoCommentHandler) LikeComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "commentID"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.LikeComment(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusNoContent, nil)
}

func (h *VideoCommentHandler) UnlikeComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "commentID"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.UnlikeComment(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusNoContent, nil)
}

func (h *VideoCommentHandler) toResponses(items []*videocomment.Comment) []*dto.VideoCommentResponse {
	out := make([]*dto.VideoCommentResponse, 0, len(items))
	for _, c := range items {
		out = append(out, h.toResponse(c))
	}
	return out
}

func (h *VideoCommentHandler) toResponse(c *videocomment.Comment) *dto.VideoCommentResponse {
	return &dto.VideoCommentResponse{
		ID:         c.ID,
		VideoID:    c.VideoID,
		UserID:     c.UserID,
		ParentID:   c.ParentID,
		Body:       c.Body,
		LikesCount: c.LikesCount,
		IsLiked:    c.IsLiked,
		CreatedAt:  c.CreatedAt,
	}
}
