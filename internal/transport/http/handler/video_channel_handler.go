package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/videochannel"
	"github.com/unowned-22/api/internal/domain/videosubscription"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
)

type VideoChannelHandler struct {
	channels videochannel.Service
	subs     videosubscription.Service
}

func NewVideoChannelHandler(ch videochannel.Service, subs videosubscription.Service) *VideoChannelHandler {
	return &VideoChannelHandler{channels: ch, subs: subs}
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
	resp := dto.ChannelResponse{ID: ch.ID, UserID: ch.UserID, Name: ch.Name, Description: ch.Description, SubscribersCount: ch.SubscribersCount, VideosCount: ch.VideosCount, CreatedAt: ch.CreatedAt}
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
	resp := dto.ChannelResponse{ID: ch.ID, UserID: ch.UserID, Name: ch.Name, Description: ch.Description, SubscribersCount: ch.SubscribersCount, VideosCount: ch.VideosCount, CreatedAt: ch.CreatedAt}
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
	resp := dto.ChannelResponse{ID: ch.ID, UserID: ch.UserID, Name: ch.Name, Description: ch.Description, SubscribersCount: ch.SubscribersCount, VideosCount: ch.VideosCount, CreatedAt: ch.CreatedAt}
	response.SendSuccess(w, http.StatusOK, resp)
}
func (h *VideoChannelHandler) ListChannelVideos(w http.ResponseWriter, r *http.Request) {
	response.SendSuccess(w, http.StatusOK, dto.VideoListResponse{Videos: []*dto.VideoResponse{}, Total: 0})
}
