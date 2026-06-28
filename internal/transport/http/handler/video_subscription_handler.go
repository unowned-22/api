package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/videochannel"
	"github.com/unowned-22/api/internal/domain/videosubscription"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
)

type VideoSubscriptionHandler struct {
	svc         videosubscription.Service
	channelRepo videochannel.Service
}

func NewVideoSubscriptionHandler(s videosubscription.Service, ch videochannel.Service) *VideoSubscriptionHandler {
	return &VideoSubscriptionHandler{svc: s, channelRepo: ch}
}
func (h *VideoSubscriptionHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	channelID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.Subscribe(r.Context(), userID, channelID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusNoContent, nil)
}
func (h *VideoSubscriptionHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	channelID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.Unsubscribe(r.Context(), userID, channelID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusNoContent, nil)
}
func (h *VideoSubscriptionHandler) GetSubscriberCount(w http.ResponseWriter, r *http.Request) {
	channelID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	ch, err := h.channelRepo.GetChannel(r.Context(), channelID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: strconv.FormatInt(ch.SubscribersCount, 10)})
}
func (h *VideoSubscriptionHandler) ListMySubscriptions(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	ids, err := h.svc.ListSubscribedChannels(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, ids)
}
