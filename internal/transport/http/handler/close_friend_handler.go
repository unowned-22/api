package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/closefriend"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type CloseFriendHandler struct {
	svc closefriend.Service
}

func NewCloseFriendHandler(svc closefriend.Service) *CloseFriendHandler {
	return &CloseFriendHandler{svc: svc}
}

func (h *CloseFriendHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	items, err := h.svc.List(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.CloseFriendResponse, 0, len(items))
	for _, id := range items {
		out = append(out, dto.CloseFriendResponse{FriendID: id})
	}
	response.SendSuccess(w, http.StatusOK, out)
}

func (h *CloseFriendHandler) Add(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	var req dto.AddCloseFriendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.SendValidationError(w, []response.ValidationFieldError{{Field: "friend_id", Message: "invalid"}})
		return
	}
	if err := h.svc.Add(r.Context(), userID, req.FriendID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusCreated, dto.CloseFriendResponse{FriendID: req.FriendID})
}

func (h *CloseFriendHandler) Remove(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	friendID, err := parseIDParam(r, "friendID")
	if err != nil {
		response.SendBadRequest(w, "invalid friend id")
		return
	}
	if err := h.svc.Remove(r.Context(), userID, friendID); err != nil {
		response.SendError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseIDParam(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, name), 10, 64)
}
