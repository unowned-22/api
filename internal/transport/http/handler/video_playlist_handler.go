package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/videoplaylist"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type VideoPlaylistHandler struct{ svc videoplaylist.Service }

func NewVideoPlaylistHandler(s videoplaylist.Service) *VideoPlaylistHandler {
	return &VideoPlaylistHandler{svc: s}
}

func (h *VideoPlaylistHandler) ListMyPlaylists(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	items, err := h.svc.ListMyPlaylists(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.PlaylistResponse, 0, len(items))
	for _, p := range items {
		out = append(out, h.toResponse(p))
	}
	response.SendSuccess(w, http.StatusOK, out)
}

func (h *VideoPlaylistHandler) CreatePlaylist(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	var req dto.CreatePlaylistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.SendValidationError(w, []response.ValidationFieldError{{Field: "title", Message: "invalid"}})
		return
	}
	p, err := h.svc.CreatePlaylist(r.Context(), userID, videoplaylist.CreateRequest{
		Title:       req.Title,
		Description: req.Description,
		Visibility:  videoplaylist.Visibility(req.Visibility),
	})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusCreated, h.toResponse(p))
}

func (h *VideoPlaylistHandler) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	userID, _ := contextx.UserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	p, err := h.svc.GetPlaylist(r.Context(), id, userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, h.toResponse(p))
}

func (h *VideoPlaylistHandler) UpdatePlaylist(w http.ResponseWriter, r *http.Request) {
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
	var req dto.UpdatePlaylistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.SendValidationError(w, []response.ValidationFieldError{{Field: "title", Message: "invalid"}})
		return
	}
	p, err := h.svc.UpdatePlaylist(r.Context(), id, userID, videoplaylist.UpdateRequest{
		Title:       req.Title,
		Description: req.Description,
		Visibility:  videoplaylist.Visibility(req.Visibility),
	})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, h.toResponse(p))
}

func (h *VideoPlaylistHandler) DeletePlaylist(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.DeletePlaylist(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusNoContent, nil)
}

func (h *VideoPlaylistHandler) AddVideoToPlaylist(w http.ResponseWriter, r *http.Request) {
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
	var req dto.AddPlaylistItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	if err := h.svc.AddVideoToPlaylist(r.Context(), id, req.VideoID, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusNoContent, nil)
}

func (h *VideoPlaylistHandler) RemoveVideoFromPlaylist(w http.ResponseWriter, r *http.Request) {
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
	videoID, err := strconv.ParseInt(chi.URLParam(r, "videoID"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid video id")
		return
	}
	if err := h.svc.RemoveVideoFromPlaylist(r.Context(), id, videoID, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusNoContent, nil)
}

func (h *VideoPlaylistHandler) ListPlaylistItems(w http.ResponseWriter, r *http.Request) {
	userID, _ := contextx.UserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	limit, offset := getPaginationQueries(r)
	items, total, err := h.svc.ListPlaylistItems(r.Context(), id, userID, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.PlaylistItemResponse, 0, len(items))
	for _, item := range items {
		out = append(out, dto.PlaylistItemResponse{
			ID:         item.ID,
			PlaylistID: item.PlaylistID,
			VideoID:    item.VideoID,
			Position:   item.Position,
			AddedAt:    item.AddedAt,
		})
	}
	response.SendSuccess(w, http.StatusOK, dto.PlaylistItemListResponse{Items: out, Total: total, Limit: limit, Offset: offset})
}

func (h *VideoPlaylistHandler) toResponse(p *videoplaylist.Playlist) dto.PlaylistResponse {
	return dto.PlaylistResponse{
		ID:          p.ID,
		UserID:      p.UserID,
		Title:       p.Title,
		Description: p.Description,
		Visibility:  string(p.Visibility),
		ItemsCount:  p.ItemsCount,
		CreatedAt:   p.CreatedAt,
	}
}
