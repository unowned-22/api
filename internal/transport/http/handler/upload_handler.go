package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/unowned-22/api/internal/contextx"
	domainstorage "github.com/unowned-22/api/internal/domain/storage"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type UploadHandler struct {
	storage     domainstorage.ObjectStorage
	bucket      string
	expiresIn   time.Duration
	userService user.UserService
}

func NewUploadHandler(storage domainstorage.ObjectStorage, bucket string, userService user.UserService) *UploadHandler {
	return &UploadHandler{
		storage:     storage,
		bucket:      bucket,
		expiresIn:   15 * time.Minute,
		userService: userService,
	}
}

func (h *UploadHandler) Presign(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	var req dto.PresignedUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		if ve, ok := errors.AsType[*validator.ValidationErrors](err); ok {
			response.SendValidationError(w, toFieldErrors(ve.Fields))
			return
		}
		response.SendBadRequest(w, "invalid request")
		return
	}

	key := path.Join(
		fmt.Sprint(userID),
		uuid.New().String(),
		req.Filename,
	)

	uploadURL, err := h.storage.PresignedPutURL(r.Context(), h.bucket, key, h.expiresIn)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.PresignedUploadResponse{
		UploadURL: uploadURL,
		Key:       key,
		ExpiresIn: int(h.expiresIn.Seconds()),
	})
}

func (h *UploadHandler) toFieldErrors(fields []validator.FieldError) []response.ValidationFieldError {
	out := make([]response.ValidationFieldError, 0, len(fields))
	for _, f := range fields {
		out = append(out, response.ValidationFieldError{
			Field:   f.Field,
			Message: f.Message,
		})
	}
	return out
}

// UploadAvatar handles POST /users/me/avatar
func (h *UploadHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	// enforce max body size 11MB
	r.Body = http.MaxBytesReader(w, r.Body, 11*1024*1024)
	mr, err := r.MultipartReader()
	if err != nil {
		response.SendBadRequest(w, "invalid multipart request")
		return
	}

	var part *multipart.Part
	for p, pErr := mr.NextPart(); pErr == nil; p, pErr = mr.NextPart() {
		if p.FormName() == "file" {
			part = p
			break
		}
	}
	if part == nil {
		response.SendBadRequest(w, "file part is required")
		return
	}
	contentType := part.Header.Get("Content-Type")
	data, err := io.ReadAll(part)
	if err != nil {
		response.SendBadRequest(w, "failed to read file")
		return
	}

	url, err := h.userService.UploadAvatar(r.Context(), userID, bytes.NewReader(data), int64(len(data)), contentType)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, map[string]string{"avatar_url": url})
}

// UploadCover handles POST /users/me/cover
func (h *UploadHandler) UploadCover(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 11*1024*1024)
	mr, err := r.MultipartReader()
	if err != nil {
		response.SendBadRequest(w, "invalid multipart request")
		return
	}

	var part *multipart.Part
	for p, pErr := mr.NextPart(); pErr == nil; p, pErr = mr.NextPart() {
		if p.FormName() == "file" {
			part = p
			break
		}
	}
	if part == nil {
		response.SendBadRequest(w, "file part is required")
		return
	}
	contentType := part.Header.Get("Content-Type")
	data, err := io.ReadAll(part)
	if err != nil {
		response.SendBadRequest(w, "failed to read file")
		return
	}

	url, err := h.userService.UploadCover(r.Context(), userID, bytes.NewReader(data), int64(len(data)), contentType)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, map[string]string{"cover_url": url})
}

// DeleteAvatar handles DELETE /users/me/avatar
func (h *UploadHandler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	if err := h.userService.DeleteAvatar(r.Context(), userID); err != nil {
		response.SendError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteCover handles DELETE /users/me/cover
func (h *UploadHandler) DeleteCover(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	if err := h.userService.DeleteCover(r.Context(), userID); err != nil {
		response.SendError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
