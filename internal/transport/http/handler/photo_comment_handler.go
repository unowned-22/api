package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/photo"
	"github.com/unowned-22/api/internal/domain/photocomment"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type PhotoCommentHandler struct {
	comments photocomment.Service
	photos   photo.Service
}

func NewPhotoCommentHandler(comments photocomment.Service, photos photo.Service) *PhotoCommentHandler {
	return &PhotoCommentHandler{comments: comments, photos: photos}
}

func (h *PhotoCommentHandler) LikePhoto(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "photoID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.photos.LikePhoto(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]string{"status": "liked"})
}

func (h *PhotoCommentHandler) UnlikePhoto(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "photoID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.photos.UnlikePhoto(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]string{"status": "unliked"})
}

func (h *PhotoCommentHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "photoID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req dto.AddCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.SendValidationError(w, []response.ValidationFieldError{{Field: "body", Message: "invalid"}})
		return
	}
	c, err := h.comments.AddComment(r.Context(), id, userID, photocomment.AddCommentInput{ParentID: req.ParentID, Body: req.Body})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := dto.CommentResponse{ID: c.ID, PhotoID: c.PhotoID, ParentID: c.ParentID, Body: c.Body, IsDeleted: c.IsDeleted, LikesCount: c.LikesCount, RepliesCount: c.RepliesCount, IsLiked: c.IsLiked, CreatedAt: c.CreatedAt.Format(time.RFC3339), UpdatedAt: c.UpdatedAt.Format(time.RFC3339)}
	if c.Author != nil && !c.IsDeleted {
		out.Author = &dto.CommentAuthorResponse{ID: c.Author.ID, FullName: c.Author.FullName, Username: c.Author.Username, AvatarURL: c.Author.AvatarURL}
	}
	response.SendSuccess(w, http.StatusCreated, out)
}

func (h *PhotoCommentHandler) ListComments(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := contextx.UserID(r.Context())
	idStr := chi.URLParam(r, "photoID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	q := r.URL.Query()
	limit := 20
	offset := 0
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	if o := q.Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}
	items, total, err := h.comments.ListComments(r.Context(), id, viewerID, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]*dto.CommentResponse, 0, len(items))
	for _, c := range items {
		cr := &dto.CommentResponse{ID: c.ID, PhotoID: c.PhotoID, ParentID: c.ParentID, Body: c.Body, IsDeleted: c.IsDeleted, LikesCount: c.LikesCount, RepliesCount: c.RepliesCount, IsLiked: c.IsLiked, CreatedAt: c.CreatedAt.Format(time.RFC3339), UpdatedAt: c.UpdatedAt.Format(time.RFC3339)}
		if c.Author != nil && !c.IsDeleted {
			cr.Author = &dto.CommentAuthorResponse{ID: c.Author.ID, FullName: c.Author.FullName, Username: c.Author.Username, AvatarURL: c.Author.AvatarURL}
		}
		out = append(out, cr)
	}
	resp := dto.PaginatedCommentsResponse{Items: out, Total: total, Limit: limit, Offset: offset}
	response.SendSuccess(w, http.StatusOK, resp)
}

func (h *PhotoCommentHandler) ListReplies(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := contextx.UserID(r.Context())
	idStr := chi.URLParam(r, "commentID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	q := r.URL.Query()
	limit := 20
	offset := 0
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	if o := q.Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}
	items, total, err := h.comments.ListReplies(r.Context(), id, viewerID, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]*dto.CommentResponse, 0, len(items))
	for _, c := range items {
		cr := &dto.CommentResponse{ID: c.ID, PhotoID: c.PhotoID, ParentID: c.ParentID, Body: c.Body, IsDeleted: c.IsDeleted, LikesCount: c.LikesCount, RepliesCount: c.RepliesCount, IsLiked: c.IsLiked, CreatedAt: c.CreatedAt.Format(time.RFC3339), UpdatedAt: c.UpdatedAt.Format(time.RFC3339)}
		if c.Author != nil && !c.IsDeleted {
			cr.Author = &dto.CommentAuthorResponse{ID: c.Author.ID, FullName: c.Author.FullName, Username: c.Author.Username, AvatarURL: c.Author.AvatarURL}
		}
		out = append(out, cr)
	}
	resp := dto.PaginatedCommentsResponse{Items: out, Total: total, Limit: limit, Offset: offset}
	response.SendSuccess(w, http.StatusOK, resp)
}

func (h *PhotoCommentHandler) EditComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "commentID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req dto.EditCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.SendValidationError(w, []response.ValidationFieldError{{Field: "body", Message: "invalid"}})
		return
	}
	c, err := h.comments.EditComment(r.Context(), id, userID, req.Body)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := dto.CommentResponse{ID: c.ID, PhotoID: c.PhotoID, ParentID: c.ParentID, Body: c.Body, IsDeleted: c.IsDeleted, LikesCount: c.LikesCount, RepliesCount: c.RepliesCount, IsLiked: c.IsLiked, CreatedAt: c.CreatedAt.Format(time.RFC3339), UpdatedAt: c.UpdatedAt.Format(time.RFC3339)}
	if c.Author != nil && !c.IsDeleted {
		out.Author = &dto.CommentAuthorResponse{ID: c.Author.ID, FullName: c.Author.FullName, Username: c.Author.Username, AvatarURL: c.Author.AvatarURL}
	}
	response.SendSuccess(w, http.StatusOK, out)
}

func (h *PhotoCommentHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "commentID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.comments.DeleteComment(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *PhotoCommentHandler) LikeComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "commentID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.comments.LikeComment(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]string{"status": "liked"})
}

func (h *PhotoCommentHandler) UnlikeComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "commentID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.comments.UnlikeComment(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]string{"status": "unliked"})
}
