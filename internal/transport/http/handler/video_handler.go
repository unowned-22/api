package handler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/community"
	"github.com/unowned-22/api/internal/domain/storage"
	domainuser "github.com/unowned-22/api/internal/domain/user"
	domainvideo "github.com/unowned-22/api/internal/domain/video"
	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
)

type VideoHandler struct {
	videos      domainvideo.Service
	communities community.Service // replaces channels (Stage 2)
	users       domainuser.UserService
	storage     storage.Storage
	cfg         *config.Config
}

func NewVideoHandler(v domainvideo.Service, c community.Service, u domainuser.UserService, s storage.Storage, cfg *config.Config) *VideoHandler {
	return &VideoHandler{videos: v, communities: c, users: u, storage: s, cfg: cfg}
}

func (h *VideoHandler) UploadVideo(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.VideoMaxFileSizeBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		response.SendBadRequest(w, "invalid multipart request")
		return
	}

	var (
		fileData    []byte
		fileName    string
		contentType string
		title       string
		description string
		category    string
		visibility  domainvideo.Visibility
		tags        []string
	)
	visibility = domainvideo.VisibilityPublic

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			response.SendBadRequest(w, "invalid multipart body")
			return
		}

		switch part.FormName() {
		case "file":
			fileName = path.Base(part.FileName())
			contentType = part.Header.Get("Content-Type")
			fileData, err = io.ReadAll(part)
			if err != nil {
				response.SendBadRequest(w, "failed to read file")
				return
			}
		case "title":
			b, _ := io.ReadAll(part)
			title = strings.TrimSpace(string(b))
		case "description":
			b, _ := io.ReadAll(part)
			description = strings.TrimSpace(string(b))
		case "category":
			b, _ := io.ReadAll(part)
			category = strings.TrimSpace(string(b))
		case "visibility":
			b, _ := io.ReadAll(part)
			visibility = domainvideo.Visibility(strings.TrimSpace(string(b)))
		case "tags":
			b, _ := io.ReadAll(part)
			_ = json.Unmarshal(b, &tags)
		}
	}

	if title == "" {
		title = fileName
	}
	if category == "" {
		category = "other"
	}
	if contentType != "video/mp4" && contentType != "video/webm" && contentType != "video/quicktime" {
		response.SendError(w, r, errs.ErrUnsupportedVideoType)
		return
	}
	if len(fileData) == 0 {
		response.SendBadRequest(w, "file is required")
		return
	}

	channel, err := h.communities.ListManageable(r.Context(), userID)
	if err != nil || len(channel) == 0 {
		response.SendError(w, r, errs.ErrCommunityNotFound)
		return
	}
	// Pick the first owned video-type community (backward-compatible behaviour).
	var communityID int64
	for _, c := range channel {
		if c.Type == community.TypeVideo && c.OwnerID == userID {
			communityID = c.ID
			break
		}
	}
	if communityID == 0 {
		response.SendNotFound(w, "no video community found; create one first")
		return
	}

	v, err := h.videos.Upload(r.Context(), domainvideo.UploadRequest{
		UserID:      userID,
		CommunityID: communityID,
		Title:       title,
		Description: description,
		Category:    category,
		Tags:        tags,
		Visibility:  visibility,
		FileName:    fileName,
		ContentType: contentType,
		SizeBytes:   int64(len(fileData)),
		Body:        bytesReader(fileData),
	})
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusCreated, h.toVideoResponse(r.Context(), v))
}

func (h *VideoHandler) VideoFeed(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	limit, offset := getPaginationQueries(r)
	items, total, err := h.videos.Feed(r.Context(), userID, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.VideoListResponse{Videos: h.toVideoResponses(r.Context(), items), Total: total})
}

func (h *VideoHandler) SearchVideos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")
	limit, offset := getPaginationQueries(r)
	items, total, err := h.videos.Search(r.Context(), q, category, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.VideoListResponse{Videos: h.toVideoResponses(r.Context(), items), Total: total})
}

func (h *VideoHandler) GetVideo(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := contextx.UserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	v, err := h.videos.GetVideo(r.Context(), id, viewerID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, h.toVideoResponse(r.Context(), v))
}

func (h *VideoHandler) UpdateVideo(w http.ResponseWriter, r *http.Request) {
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
	var req dto.UpdateVideoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	v, err := h.videos.UpdateMeta(r.Context(), id, userID, domainvideo.UpdateMetaRequest{
		Title:        req.Title,
		Description:  req.Description,
		Category:     req.Category,
		Tags:         req.Tags,
		Visibility:   domainvideo.Visibility(req.Visibility),
		ThumbnailKey: req.ThumbnailKey,
	})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, h.toVideoResponse(r.Context(), v))
}

func (h *VideoHandler) DeleteVideo(w http.ResponseWriter, r *http.Request) {
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
	if err := h.videos.Delete(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusNoContent, nil)
}

func (h *VideoHandler) RecordView(w http.ResponseWriter, r *http.Request) {
	var userID *int64
	if uid, ok := contextx.UserID(r.Context()); ok {
		userID = &uid
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.videos.RecordView(r.Context(), id, userID, hashIP(r.RemoteAddr)); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusNoContent, nil)
}

func (h *VideoHandler) LikeVideo(w http.ResponseWriter, r *http.Request) {
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
	if err := h.videos.LikeVideo(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusNoContent, nil)
}

func (h *VideoHandler) UnlikeVideo(w http.ResponseWriter, r *http.Request) {
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
	if err := h.videos.UnlikeVideo(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusNoContent, nil)
}

func (h *VideoHandler) toVideoResponse(ctx context.Context, v *domainvideo.Video) dto.VideoResponse {
	resp := dto.VideoResponse{
		ID:                 v.ID,
		CommunityID:        v.CommunityID,
		ChannelID:          v.CommunityID, // backward-compat alias
		UserID:             v.UserID,
		Title:              v.Title,
		Description:        v.Description,
		Category:           v.Category,
		Tags:               v.Tags,
		Visibility:         string(v.Visibility),
		Status:             string(v.Status),
		CoverURL:           v.CoverKey,
		ThumbnailURL:       v.ThumbnailKey,
		HLSURL:             v.HLSKey,
		DurationSec:        v.DurationSec,
		Width:              v.Width,
		Height:             v.Height,
		ViewsCount:         v.ViewsCount,
		LikesCount:         v.LikesCount,
		CommentsCount:      v.CommentsCount,
		IsLiked:            v.IsLiked,
		CreatedAt:          v.CreatedAt,
		ProcessingStage:    v.ProcessingStage,
		ProcessingProgress: v.ProcessingProgress,
	}
	if v.CoverKey != "" {
		if url, err := h.storage.PresignURL(ctx, h.cfg.MinIOBucket, v.CoverKey, time.Hour); err == nil {
			resp.CoverURL = url
		}
	}
	if v.ThumbnailKey != "" {
		if url, err := h.storage.PresignURL(ctx, h.cfg.MinIOBucket, v.ThumbnailKey, time.Hour); err == nil {
			resp.ThumbnailURL = url
		}
	}
	if v.HLSKey != "" {
		if url, err := h.storage.PresignURL(ctx, h.cfg.MinIOBucket, v.HLSKey, time.Hour); err == nil {
			resp.HLSURL = url
		}
	}
	return resp
}

func (h *VideoHandler) toVideoResponses(ctx context.Context, items []*domainvideo.Video) []*dto.VideoResponse {
	out := make([]*dto.VideoResponse, 0, len(items))
	for _, v := range items {
		resp := h.toVideoResponse(ctx, v)
		out = append(out, &resp)
	}
	return out
}

func hashIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	sum := sha256.Sum256([]byte(host))
	return fmt.Sprintf("%x", sum)
}

type byteReader struct {
	b []byte
	i int
}

func bytesReader(b []byte) io.Reader { return &byteReader{b: b} }

func (r *byteReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

// ── Stage 2 additions ─────────────────────────────────────────────────────────

// ListByCommunity  GET /api/v1/communities/{id}/videos
func (h *VideoHandler) ListByCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid community id")
		return
	}
	viewerID, _ := contextx.UserID(r.Context())
	limit, offset := getPaginationQueries(r)
	items, total, err := h.videos.ListByCommunity(r.Context(), communityID, viewerID, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.VideoListResponse{Videos: h.toVideoResponses(r.Context(), items), Total: total})
}

// ListDrafts  GET /api/v1/communities/{id}/videos/drafts
// Only admins/owners of the community can see this.
func (h *VideoHandler) ListDrafts(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	communityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid community id")
		return
	}
	limit, offset := getPaginationQueries(r)
	items, total, svcErr := h.videos.ListDrafts(r.Context(), communityID, requesterID, limit, offset)
	if svcErr != nil {
		response.SendError(w, r, svcErr)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.VideoListResponse{Videos: h.toVideoResponses(r.Context(), items), Total: total})
}

// PublishVideo  POST /api/v1/videos/{id}/publish
func (h *VideoHandler) PublishVideo(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	videoID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid video id")
		return
	}
	var req dto.PublishVideoRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // body is optional
	if svcErr := h.videos.Publish(r.Context(), videoID, requesterID, req.Targets); svcErr != nil {
		response.SendError(w, r, svcErr)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "published"})
}

// UnpublishVideo  POST /api/v1/videos/{id}/unpublish
func (h *VideoHandler) UnpublishVideo(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	videoID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid video id")
		return
	}
	if svcErr := h.videos.Unpublish(r.Context(), videoID, requesterID); svcErr != nil {
		response.SendError(w, r, svcErr)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "unpublished"})
}

// BoostVideo  POST /api/v1/videos/{id}/boost
// Stub — persists boosted_until but has no effect on ranking/billing.
// TODO: implement billing/ranking when the promotion subsystem is ready.
//
//	See TASK-communities §4, §10 and AGENTS.md "Boost / promotion stub".
func (h *VideoHandler) BoostVideo(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	videoID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid video id")
		return
	}
	var req dto.BoostVideoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Hours <= 0 {
		req.Hours = 24 // sensible default
	}
	if svcErr := h.videos.Boost(r.Context(), videoID, requesterID, req.Hours); svcErr != nil {
		response.SendError(w, r, svcErr)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "boost scheduled (stub)"})
}
