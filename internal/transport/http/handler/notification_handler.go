package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/pagination"
	"github.com/unowned-22/api/internal/transport/http/response"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

type NotificationHandler struct {
	svc notification.Service
	hub *ws.Hub
}

func NewNotificationHandler(s notification.Service, h *ws.Hub) *NotificationHandler {
	return &NotificationHandler{svc: s, hub: h}
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	q := pagination.ParseQuery(r)
	items, total, err := h.svc.ListMy(r.Context(), userID, q)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, pagination.BuildResponse(items, q.Page, q.Limit, total))
}

func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	c, err := h.svc.UnreadCount(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]int64{"unread": c})
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.MarkRead(r.Context(), userID, id); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	if err := h.svc.MarkAllRead(r.Context(), userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *NotificationHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.hub.Register(userID, conn)
	defer func() {
		h.hub.Unregister(userID, conn)
		conn.Close()
	}()

	// keep connection alive; read loop to detect client close
	for {
		if _, _, err := conn.NextReader(); err != nil {
			break
		}
	}
}
