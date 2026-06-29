package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	domainvideo "github.com/unowned-22/api/internal/domain/video"
	"github.com/unowned-22/api/internal/domain/videochannel"
	"github.com/unowned-22/api/internal/domain/videosubscription"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type VideoChannelHandler struct {
	channels videochannel.Service
	subs     videosubscription.Service
	videos   domainvideo.Service
}

func NewVideoChannelHandler(ch videochannel.Service, subs videosubscription.Service, videos domainvideo.Service) *VideoChannelHandler {
	return &VideoChannelHandler{channels: ch, subs: subs, videos: videos}
}
func (h *VideoChannelHandler) CreateMyChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	var req dto.CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	ch, err := h.channels.CreateChannel(r.Context(), userID, videochannel.CreateRequest{Name: req.Name, Description: req.Description})
	if err != nil {
		var validationErrs *validator.ValidationErrors
		if errors.As(err, &validationErrs) {
			response.SendValidationError(w, []response.ValidationFieldError{{Field: "name", Message: validationErrs.Error()}})
			return
		}
		response.SendError(w, r, err)
		return
	}
	resp := h.toChannelResponse(ch)
	response.SendSuccess(w, http.StatusCreated, resp)
}

func (h *VideoChannelHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6*1024*1024)
	mr, err := r.MultipartReader()
	if err != nil {
		response.SendBadRequest(w, "invalid multipart request")
		return
	}
	var (
		fileData    []byte
		contentType string
	)
	for {
		part, pErr := mr.NextPart()
		if pErr == io.EOF {
			break
		}
		if pErr != nil {
			response.SendBadRequest(w, "invalid multipart body")
			return
		}
		if part.FormName() == "file" {
			contentType = part.Header.Get("Content-Type")
			fileData, err = io.ReadAll(part)
			if err != nil {
				response.SendBadRequest(w, "failed to read file")
				return
			}
			break
		}
	}
	if len(fileData) == 0 {
		response.SendBadRequest(w, "file is required")
		return
	}
	ch, err := h.channels.UploadAvatar(r.Context(), userID, fileData, int64(len(fileData)), contentType)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.ChannelAvatarResponse{AvatarURL: ch.AvatarKey})
}

func (h *VideoChannelHandler) UploadBanner(w http.ResponseWriter, r *http.Request) {
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
	var (
		fileData    []byte
		contentType string
	)
	for {
		part, pErr := mr.NextPart()
		if pErr == io.EOF {
			break
		}
		if pErr != nil {
			response.SendBadRequest(w, "invalid multipart body")
			return
		}
		if part.FormName() == "file" {
			contentType = part.Header.Get("Content-Type")
			fileData, err = io.ReadAll(part)
			if err != nil {
				response.SendBadRequest(w, "failed to read file")
				return
			}
			break
		}
	}
	if len(fileData) == 0 {
		response.SendBadRequest(w, "file is required")
		return
	}
	ch, err := h.channels.UploadBanner(r.Context(), userID, fileData, int64(len(fileData)), contentType)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.ChannelBannerResponse{BannerURL: ch.BannerKey})
}

func (h *VideoChannelHandler) GetMyChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	ch, err := h.channels.GetOrCreate(r.Context(), userID, "")
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	resp := h.toChannelResponse(ch)
	response.SendSuccess(w, http.StatusOK, resp)
}
func (h *VideoChannelHandler) UpdateMyChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	var req dto.UpdateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	ch, err := h.channels.GetOrCreate(r.Context(), userID, "")
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	ch, err = h.channels.UpdateChannel(r.Context(), ch.ID, userID, videochannel.UpdateRequest{Name: req.Name, Description: req.Description, AvatarKey: req.AvatarKey, BannerKey: req.BannerKey})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	resp := h.toChannelResponse(ch)
	response.SendSuccess(w, http.StatusOK, resp)
}
func (h *VideoChannelHandler) GetChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	ch, err := h.channels.GetChannel(r.Context(), id)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	resp := h.toChannelResponse(ch)
	response.SendSuccess(w, http.StatusOK, resp)
}

func (h *VideoChannelHandler) toChannelResponse(ch *videochannel.Channel) dto.ChannelResponse {
	return dto.ChannelResponse{ID: ch.ID, UserID: ch.UserID, Name: ch.Name, Description: ch.Description, AvatarURL: ch.AvatarKey, BannerURL: ch.BannerKey, SubscribersCount: ch.SubscribersCount, VideosCount: ch.VideosCount, CreatedAt: ch.CreatedAt}
}

func (h *VideoChannelHandler) ListChannelVideos(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := contextx.UserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	limit, offset := getPaginationQueries(r)
	items, total, err := h.videos.ListByChannel(r.Context(), id, viewerID, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]*dto.VideoResponse, 0, len(items))
	for _, v := range items {
		r := dto.VideoResponse{
			ID:            v.ID,
			ChannelID:     v.ChannelID,
			UserID:        v.UserID,
			Title:         v.Title,
			Description:   v.Description,
			Category:      v.Category,
			Tags:          v.Tags,
			Visibility:    string(v.Visibility),
			Status:        string(v.Status),
			ThumbnailURL:  v.ThumbnailKey,
			HLSURL:        v.HLSKey,
			DurationSec:   v.DurationSec,
			Width:         v.Width,
			Height:        v.Height,
			ViewsCount:    v.ViewsCount,
			LikesCount:    v.LikesCount,
			CommentsCount: v.CommentsCount,
			IsLiked:       v.IsLiked,
			CreatedAt:     v.CreatedAt,
		}
		out = append(out, &r)
	}
	response.SendSuccess(w, http.StatusOK, dto.VideoListResponse{Videos: out, Total: total})
}
