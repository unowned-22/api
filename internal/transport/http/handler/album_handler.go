package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/album"
	"github.com/unowned-22/api/internal/domain/photo"
	"github.com/unowned-22/api/internal/domain/profile"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type AlbumHandler struct {
	albums   album.Service
	photos   photo.Service
	profiles profile.Service
}

func NewAlbumHandler(albums album.Service, photos photo.Service, profiles profile.Service) *AlbumHandler {
	return &AlbumHandler{albums: albums, photos: photos, profiles: profiles}
}

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
	vis := album.Visibility(req.Visibility)
	if req.Visibility == "" {
		vis = album.VisibilityEveryone
	}
	a, err := h.albums.Create(r.Context(), userID, album.CreateInput{Title: req.Title, Description: req.Description, Visibility: vis, HiddenFrom: req.HiddenFrom})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	resp := h.toAlbumResponse(r.Context(), a)
	response.SendSuccess(w, http.StatusCreated, resp)
}

func (h *AlbumHandler) ListMyAlbums(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	limit, offset := getPaginationQueries(r)
	items, _, err := h.albums.ListUserAlbums(r.Context(), userID, userID, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := h.toAlbumResponses(r.Context(), items)
	response.SendSuccess(w, http.StatusOK, out)
}

func (h *AlbumHandler) ListUserAlbumsByUsername(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := contextx.UserID(r.Context())
	username := chi.URLParam(r, "username")
	if username == "" {
		response.SendBadRequest(w, "username is required")
		return
	}
	p, err := h.profiles.GetPublicProfile(r.Context(), viewerID, username)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	limit, offset := getPaginationQueries(r)
	items, _, err := h.albums.ListUserAlbums(r.Context(), p.ID, viewerID, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := h.toAlbumResponses(r.Context(), items)
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
	resp := h.toAlbumResponse(r.Context(), a)
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
	vis := album.Visibility(req.Visibility)
	if req.Visibility == "" {
		vis = album.VisibilityEveryone
	}
	a, err := h.albums.Update(r.Context(), id, userID, album.UpdateInput{Title: req.Title, Description: req.Description, Visibility: vis, HiddenFrom: req.HiddenFrom})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	resp := h.toAlbumResponse(r.Context(), a)
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
	photos, _, err := h.photos.ListAlbumPhotos(r.Context(), id, viewerID, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.PhotoResponse, 0, len(photos))
	for _, p := range photos {
		out = append(out, dto.PhotoResponse{ID: p.ID, AlbumID: p.AlbumID, DisplayName: p.DisplayName, URL: p.URL, SizeBytes: p.SizeBytes, Width: p.Width, Height: p.Height, MimeType: p.MimeType, Visibility: string(p.Visibility), LikesCount: p.LikesCount, CommentsCount: p.CommentsCount, DeviceName: derefString(p.DeviceName), DeviceOS: derefString(p.DeviceOS), DeviceType: derefString(p.DeviceType), Latitude: p.Latitude, Longitude: p.Longitude, LocationName: derefStringPtr(p.LocationName), ExifData: p.ExifData, CreatedAt: p.CreatedAt.Format(time.RFC3339)})
	}
	response.SendSuccess(w, http.StatusOK, out)
}

func (h *AlbumHandler) toAlbumResponse(ctx context.Context, a *album.Album) dto.AlbumResponse {
	resp := dto.AlbumResponse{
		ID:          a.ID,
		Title:       a.Title,
		Description: a.Description,
		Visibility:  string(a.Visibility),
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
	}
	if a.CoverPhotoID != nil {
		urls, err := h.photos.GetURLsByIDs(ctx, []int64{*a.CoverPhotoID})
		if err == nil {
			if url, ok := urls[*a.CoverPhotoID]; ok {
				resp.CoverURL = &url
			}
		}
	}
	return resp
}

func (h *AlbumHandler) toAlbumResponses(ctx context.Context, albums []*album.Album) []dto.AlbumResponse {
	ids := make([]int64, 0, len(albums))
	for _, a := range albums {
		if a.CoverPhotoID != nil {
			ids = append(ids, *a.CoverPhotoID)
		}
	}
	urls, err := h.photos.GetURLsByIDs(ctx, ids)
	if err != nil {
		urls = map[int64]string{}
	}

	out := make([]dto.AlbumResponse, 0, len(albums))
	for _, a := range albums {
		resp := dto.AlbumResponse{
			ID:          a.ID,
			Title:       a.Title,
			Description: a.Description,
			Visibility:  string(a.Visibility),
			CreatedAt:   a.CreatedAt.Format(time.RFC3339),
		}
		if a.CoverPhotoID != nil {
			if url, ok := urls[*a.CoverPhotoID]; ok {
				resp.CoverURL = &url
			}
		}
		out = append(out, resp)
	}
	return out
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefStringPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
