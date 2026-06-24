package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/album"
	"github.com/unowned-22/api/internal/domain/photo"
	"github.com/unowned-22/api/internal/pkg/uaparser"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type PhotoHandler struct {
	photos photo.Service
	albums album.Service
}

func NewPhotoHandler(photos photo.Service, albums album.Service) *PhotoHandler {
	return &PhotoHandler{photos: photos, albums: albums}
}

// UploadPhoto handles POST /api/v1/photos
func (h *PhotoHandler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	// limit body to 20MB
	r.Body = http.MaxBytesReader(w, r.Body, 20*1024*1024)
	mr, err := r.MultipartReader()
	if err != nil {
		response.SendBadRequest(w, "invalid multipart request")
		return
	}

	var part io.Reader
	var filename string
	var contentType string
	for p, pErr := mr.NextPart(); pErr == nil; p, pErr = mr.NextPart() {
		if p.FormName() == "file" {
			b, _ := io.ReadAll(p)
			part = bytes.NewReader(b)
			filename = path.Base(p.FileName())
			contentType = p.Header.Get("Content-Type")
			break
		}
	}
	if part == nil {
		response.SendBadRequest(w, "file part required")
		return
	}

	// optional album_id field in form
	var albumID *int64
	if v := r.FormValue("album_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			albumID = &id
		}
	}

	// validate content type
	allowed := map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}
	if !allowed[contentType] {
		response.SendBadRequest(w, "unsupported content type")
		return
	}

	// read into buffer to get size
	data, err := io.ReadAll(part)
	if err != nil {
		response.SendBadRequest(w, "failed to read file")
		return
	}
	if len(data) == 0 || len(data) > 10*1024*1024 {
		response.SendBadRequest(w, "file size invalid")
		return
	}

	// parse user-agent for device info
	di := uaparser.Parse(r.Header.Get("User-Agent"))
	deviceName := di.Browser
	deviceOS := di.OS
	deviceType := di.DeviceType

	p, err := h.photos.Upload(r.Context(), userID, photo.UploadInput{Reader: bytes.NewReader(data), Size: int64(len(data)), Filename: filename, ContentType: contentType, AlbumID: albumID, DeviceName: &deviceName, DeviceOS: &deviceOS, DeviceType: &deviceType})
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	resp := dto.PhotoResponse{
		ID: p.ID, AlbumID: p.AlbumID, DisplayName: p.DisplayName, URL: p.URL, SizeBytes: p.SizeBytes, Width: p.Width, Height: p.Height, MimeType: p.MimeType, Visibility: string(p.Visibility), CreatedAt: p.CreatedAt.Format(time.RFC3339),
	}
	response.SendSuccess(w, http.StatusCreated, resp)
}

// ListMyPhotos handles GET /api/v1/photos
func (h *PhotoHandler) ListMyPhotos(w http.ResponseWriter, r *http.Request) {
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

	items, _, err := h.photos.ListUserPhotos(r.Context(), userID, userID, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.PhotoResponse, 0, len(items))
	for _, p := range items {
		out = append(out, dto.PhotoResponse{ID: p.ID, AlbumID: p.AlbumID, DisplayName: p.DisplayName, URL: p.URL, SizeBytes: p.SizeBytes, Width: p.Width, Height: p.Height, MimeType: p.MimeType, Visibility: string(p.Visibility), CreatedAt: p.CreatedAt.Format(time.RFC3339)})
	}
	response.SendSuccess(w, http.StatusOK, out)
}

// GetPhoto handles GET /api/v1/photos/{photoID}
func (h *PhotoHandler) GetPhoto(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := contextx.UserID(r.Context())
	idStr := chi.URLParam(r, "photoID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	p, err := h.photos.GetPhoto(r.Context(), id, viewerID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	resp := dto.PhotoResponse{ID: p.ID, AlbumID: p.AlbumID, DisplayName: p.DisplayName, URL: p.URL, SizeBytes: p.SizeBytes, Width: p.Width, Height: p.Height, MimeType: p.MimeType, Visibility: string(p.Visibility), CreatedAt: p.CreatedAt.Format(time.RFC3339)}
	response.SendSuccess(w, http.StatusOK, resp)
}

// UpdatePhotoMeta handles PATCH /api/v1/photos/{photoID}
func (h *PhotoHandler) UpdatePhotoMeta(w http.ResponseWriter, r *http.Request) {
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
	var req dto.UpdatePhotoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.SendValidationError(w, []response.ValidationFieldError{{Field: "", Message: "validation failed"}})
		return
	}
	if err := h.photos.UpdateMeta(r.Context(), id, userID, req.DisplayName, photo.Visibility(req.Visibility), req.HiddenFrom); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]string{"status": "ok"})
}

// MovePhotoToAlbum handles PATCH /api/v1/photos/{photoID}/move
func (h *PhotoHandler) MovePhotoToAlbum(w http.ResponseWriter, r *http.Request) {
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
	var req dto.MovePhotoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	if err := h.photos.MoveToAlbum(r.Context(), id, userID, req.AlbumID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeletePhoto handles DELETE /api/v1/photos/{photoID}
func (h *PhotoHandler) DeletePhoto(w http.ResponseWriter, r *http.Request) {
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
	if err := h.photos.Delete(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]string{"status": "deleted"})
}
