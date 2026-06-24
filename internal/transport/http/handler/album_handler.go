package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/album"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type AlbumHandler struct {
	albums album.Service
}

func NewAlbumHandler(albums album.Service) *AlbumHandler { return &AlbumHandler{albums: albums} }

func (h *AlbumHandler) CreateAlbum(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	var req dto.CreateAlbumRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.SendValidationError(w, []response.ValidationFieldError{{Field: "", Message: "validation failed"}})
		return
	}
	a, err := h.albums.Create(r.Context(), userID, album.CreateInput{Title: req.Title, Description: req.Description, Visibility: album.Visibility(req.Visibility), HiddenFrom: req.HiddenFrom})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	resp := dto.AlbumResponse{ID: a.ID, Title: a.Title, Description: a.Description, Visibility: string(a.Visibility), CreatedAt: a.CreatedAt.Format(time.RFC3339)}
	response.SendSuccess(w, http.StatusCreated, resp)
}

func (h *AlbumHandler) ListMyAlbums(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
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
	items, _, err := h.albums.ListUserAlbums(r.Context(), userID, userID, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.AlbumResponse, 0, len(items))
	for _, a := range items {
		out = append(out, dto.AlbumResponse{ID: a.ID, Title: a.Title, Description: a.Description, Visibility: string(a.Visibility), CreatedAt: a.CreatedAt.Format(time.RFC3339)})
	}
	response.SendSuccess(w, http.StatusOK, out)
}

func (h *AlbumHandler) GetAlbum(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := contextx.UserID(r.Context())
	idStr := chi.URLParam(r, "albumID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	a, err := h.albums.Get(r.Context(), id, viewerID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	resp := dto.AlbumResponse{ID: a.ID, Title: a.Title, Description: a.Description, Visibility: string(a.Visibility), CreatedAt: a.CreatedAt.Format(time.RFC3339)}
	response.SendSuccess(w, http.StatusOK, resp)
}

func (h *AlbumHandler) UpdateAlbum(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "albumID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req dto.UpdateAlbumRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.SendValidationError(w, []response.ValidationFieldError{{Field: "", Message: "validation failed"}})
		return
	}
	a, err := h.albums.Update(r.Context(), id, userID, album.UpdateInput{Title: req.Title, Description: req.Description, Visibility: album.Visibility(req.Visibility), HiddenFrom: req.HiddenFrom})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	resp := dto.AlbumResponse{ID: a.ID, Title: a.Title, Description: a.Description, Visibility: string(a.Visibility), CreatedAt: a.CreatedAt.Format(time.RFC3339)}
	response.SendSuccess(w, http.StatusOK, resp)
}

func (h *AlbumHandler) DeleteAlbum(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "albumID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.albums.Delete(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *AlbumHandler) SetAlbumCover(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "albumID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req dto.SetAlbumCoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	if err := h.albums.SetCover(r.Context(), id, userID, req.PhotoID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AlbumHandler) ListAlbumPhotos(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := contextx.UserID(r.Context())
	idStr := chi.URLParam(r, "albumID")
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
	// Check album visibility via albums.Get
	if _, err := h.albums.Get(r.Context(), id, viewerID); err != nil {
		response.SendError(w, r, err)
		return
	}
	items, _, err := h.albums.ListUserAlbums(r.Context(), 0, viewerID, limit, offset) // placeholder; service exposes ListAlbumPhotos via photo service in real impl
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	// For now, return empty photo list structure per API contract
	response.SendSuccess(w, http.StatusOK, items)
}
