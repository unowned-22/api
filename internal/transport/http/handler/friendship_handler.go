package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/pagination"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type FriendshipHandler struct {
	svc friendship.Service
}

func NewFriendshipHandler(svc friendship.Service) *FriendshipHandler {
	return &FriendshipHandler{svc: svc}
}

// POST /api/v1/friends/requests
func (h *FriendshipHandler) SendRequest(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	var req dto.SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		if ve, ok := err.(*validator.ValidationErrors); ok {
			response.SendValidationError(w, h.toFieldErrors(ve.Fields))
			return
		}
		response.SendBadRequest(w, "invalid request")
		return
	}

	f, err := h.svc.SendRequest(r.Context(), userID, req.AddresseeID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	resp := dto.FriendshipResponse{ID: f.ID, RequesterID: f.RequesterID, AddresseeID: f.AddresseeID, Status: string(f.Status), CreatedAt: f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), UpdatedAt: f.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")}
	response.SendSuccess(w, http.StatusCreated, resp)
}

// POST /api/v1/friends/requests/{id}/accept
func (h *FriendshipHandler) Accept(w http.ResponseWriter, r *http.Request) {
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
	f, err := h.svc.Accept(r.Context(), userID, id)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	resp := dto.FriendshipResponse{ID: f.ID, RequesterID: f.RequesterID, AddresseeID: f.AddresseeID, Status: string(f.Status), CreatedAt: f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), UpdatedAt: f.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")}
	response.SendSuccess(w, http.StatusOK, resp)
}

// POST /api/v1/friends/requests/{id}/reject
func (h *FriendshipHandler) Reject(w http.ResponseWriter, r *http.Request) {
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
	f, err := h.svc.Reject(r.Context(), userID, id)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	resp := dto.FriendshipResponse{ID: f.ID, RequesterID: f.RequesterID, AddresseeID: f.AddresseeID, Status: string(f.Status), CreatedAt: f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), UpdatedAt: f.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")}
	response.SendSuccess(w, http.StatusOK, resp)
}

// POST /api/v1/friends/requests/{id}/cancel
func (h *FriendshipHandler) Cancel(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.Cancel(r.Context(), userID, id); err != nil {
		response.SendError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/v1/friends/{id}
func (h *FriendshipHandler) Remove(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.Remove(r.Context(), userID, id); err != nil {
		response.SendError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/friends
func (h *FriendshipHandler) ListFriends(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	q := pagination.ParseQuery(r)
	items, total, err := h.svc.ListFriends(r.Context(), userID, q)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.FriendshipResponse, 0, len(items))
	for _, f := range items {
		out = append(out, dto.FriendshipResponse{ID: f.ID, RequesterID: f.RequesterID, AddresseeID: f.AddresseeID, Status: string(f.Status), CreatedAt: f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), UpdatedAt: f.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")})
	}
	response.SendSuccess(w, http.StatusOK, pagination.BuildResponse(out, q.Page, q.Limit, total))
}

// GET /api/v1/friends/requests/incoming
func (h *FriendshipHandler) ListIncoming(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	q := pagination.ParseQuery(r)
	items, total, err := h.svc.ListIncomingRequests(r.Context(), userID, q)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.FriendshipResponse, 0, len(items))
	for _, f := range items {
		out = append(out, dto.FriendshipResponse{ID: f.ID, RequesterID: f.RequesterID, AddresseeID: f.AddresseeID, Status: string(f.Status), CreatedAt: f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), UpdatedAt: f.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")})
	}
	response.SendSuccess(w, http.StatusOK, pagination.BuildResponse(out, q.Page, q.Limit, total))
}

// GET /api/v1/friends/requests/outgoing
func (h *FriendshipHandler) ListOutgoing(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	q := pagination.ParseQuery(r)
	items, total, err := h.svc.ListOutgoingRequests(r.Context(), userID, q)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.FriendshipResponse, 0, len(items))
	for _, f := range items {
		out = append(out, dto.FriendshipResponse{ID: f.ID, RequesterID: f.RequesterID, AddresseeID: f.AddresseeID, Status: string(f.Status), CreatedAt: f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), UpdatedAt: f.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")})
	}
	response.SendSuccess(w, http.StatusOK, pagination.BuildResponse(out, q.Page, q.Limit, total))
}

func (h *FriendshipHandler) toFieldErrors(fields []validator.FieldError) []response.ValidationFieldError {
	out := make([]response.ValidationFieldError, 0, len(fields))
	for _, f := range fields {
		out = append(out, response.ValidationFieldError{Field: f.Field, Message: f.Message})
	}
	return out
}
