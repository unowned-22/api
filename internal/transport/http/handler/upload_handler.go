package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/unowned-22/api/internal/contextx"
	domainstorage "github.com/unowned-22/api/internal/domain/storage"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type UploadHandler struct {
	storage   domainstorage.ObjectStorage
	bucket    string
	expiresIn time.Duration
}

func NewUploadHandler(storage domainstorage.ObjectStorage, bucket string) *UploadHandler {
	return &UploadHandler{
		storage:   storage,
		bucket:    bucket,
		expiresIn: 15 * time.Minute,
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
