package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/media"
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

func NewUploadHandler(storage domainstorage.ObjectStorage, publicBucket string, userService user.UserService) *UploadHandler {
	return &UploadHandler{
		storage:     storage,
		bucket:      publicBucket,
		expiresIn:   15 * time.Minute,
		userService: userService,
	}
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

	var fileData []byte
	var contentType string
	var cropReq dto.CoverCropRequest
	hasCrop := false

	for {
		p, pErr := mr.NextPart()
		if pErr == io.EOF {
			break
		}
		if pErr != nil {
			response.SendBadRequest(w, "failed to read multipart")
			return
		}

		switch p.FormName() {
		case "file":
			contentType = p.Header.Get("Content-Type")
			fileData, err = io.ReadAll(p)
			if err != nil {
				response.SendBadRequest(w, "failed to read file")
				return
			}
		case "crop":
			raw, rErr := io.ReadAll(p)
			if rErr != nil {
				response.SendBadRequest(w, "failed to read crop field")
				return
			}
			if jErr := json.Unmarshal(raw, &cropReq); jErr != nil {
				response.SendBadRequest(w, "invalid crop JSON")
				return
			}
			hasCrop = true
		}
	}

	if len(fileData) == 0 {
		response.SendBadRequest(w, "file part is required")
		return
	}
	if !hasCrop {
		response.SendBadRequest(w, "crop field is required")
		return
	}

	mob := user.CropRect{
		X:      cropReq.Mobile.X,
		Y:      cropReq.Mobile.Y,
		Width:  cropReq.Mobile.Width,
		Height: cropReq.Mobile.Height,
	}

	desc := user.CropRect{
		X:      cropReq.Desktop.X,
		Y:      cropReq.Desktop.Y,
		Width:  cropReq.Desktop.Width,
		Height: cropReq.Desktop.Height,
	}

	result, err := h.userService.UploadCover(
		r.Context(), userID,
		bytes.NewReader(fileData), int64(len(fileData)), contentType,
		user.CoverCrop{
			Mobile:  mob,
			Desktop: desc,
		},
	)

	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.CoverUploadResponse{
		OriginalURL: result.CoverURL,
		MobileURL:   result.CoverMobileURL,
		DesktopURL:  result.CoverDesktopURL,
	})
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

// UploadStoryMedia handles POST /stories/media.
// Images are validated by sniffing actual bytes via media.DetectFormat.
// Video format validation continues to trust the client Content-Type header
// (video format detection is out of scope — bytes pass through unprocessed).
func (h *UploadHandler) UploadStoryMedia(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	// enforce max body size 50MB
	r.Body = http.MaxBytesReader(w, r.Body, 50*1024*1024)
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

	mediaType, err := classifyStoryMedia(data, contentType)
	if err != nil {
		response.SendBadRequest(w, err.Error())
		return
	}

	key := path.Join("stories", fmt.Sprint(userID), uuid.New().String(), part.FileName())
	uploadReq := domainstorage.UploadRequest{
		Bucket:      h.bucket,
		Key:         key,
		Body:        bytes.NewReader(data),
		Size:        int64(len(data)),
		ContentType: contentType,
	}
	if err := h.storage.Upload(r.Context(), uploadReq); err != nil {
		response.SendError(w, r, err)
		return
	}

	var url string
	if s, ok := h.storage.(domainstorage.Storage); ok {
		u, err := s.PresignURL(r.Context(), h.bucket, key, 15*time.Minute)
		if err != nil {
			response.SendError(w, r, err)
			return
		}
		url = u
	} else {
		u, err := h.storage.GetURL(r.Context(), h.bucket, key, 15*time.Minute)
		if err != nil {
			response.SendError(w, r, err)
			return
		}
		url = u
	}

	response.SendSuccess(w, http.StatusOK, dto.StoryMediaResponse{URL: url, Key: key, MediaType: mediaType})
}

// classifyStoryMedia decides whether a story upload is an image or video and
// validates that the format is supported.
//
// For images: format is detected from the actual file bytes via
// media.DetectFormat so that client-mislabeled files (e.g. AVIF sent as
// image/jpeg) are still correctly identified.
//
// For videos: the client-supplied Content-Type is trusted because video
// format detection from bytes is out of scope for this task.
func classifyStoryMedia(data []byte, contentType string) (mediaType string, err error) {
	allowedVideoTypes := map[string]bool{
		"video/mp4":       true,
		"video/quicktime": true,
		"video/webm":      true,
	}

	if strings.HasPrefix(contentType, "video/") {
		if !allowedVideoTypes[contentType] {
			return "", fmt.Errorf("unsupported video content type")
		}
		return "video", nil
	}

	// For images, sniff the actual bytes.
	allowedImageFormats := map[media.Format]bool{
		media.FormatJPEG: true,
		media.FormatPNG:  true,
		media.FormatWebP: true,
		media.FormatGIF:  true,
	}

	f, fErr := media.DetectFormat(data)
	if fErr != nil || !allowedImageFormats[f] {
		return "", fmt.Errorf("unsupported content type")
	}
	return "image", nil
}
