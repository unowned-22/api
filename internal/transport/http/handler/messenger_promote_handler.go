package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// PromoteToCommunity  POST /api/v1/conversations/{id}/promote-to-community
//
// NOTE: this method is defined on *MessengerHandler in a separate file
// (Go allows splitting a type's methods across files in the same package).
// If the existing messenger handler struct field for the service is not
// named `svc`, adjust `h.svc.PromoteToCommunity(...)` below to match
// (see internal/transport/http/handler/messenger_handler.go).
func (h *MessengerHandler) PromoteToCommunity(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	conversationID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid conversation id")
		return
	}

	var req dto.PromoteToCommunityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if req.Type == "" {
		req.Type = "general"
	}
	if req.Visibility == "" {
		req.Visibility = "public"
	}

	communityID, svcErr := h.svc.PromoteToCommunity(r.Context(), requesterID, conversationID, req.Type, req.Visibility)
	if svcErr != nil {
		response.SendError(w, r, svcErr)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.PromoteToCommunityResponse{CommunityID: communityID})
}
