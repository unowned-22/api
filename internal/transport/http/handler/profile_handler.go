package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/profile"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
)

type ProfileHandler struct {
	svc profile.Service
}

func NewProfileHandler(svc profile.Service) *ProfileHandler {
	return &ProfileHandler{svc: svc}
}

func toPublicProfileDTO(p *profile.PublicProfile) *dto.PublicProfileResponse {
	var created string
	if !p.CreatedAt.IsZero() {
		created = p.CreatedAt.UTC().Format(time.RFC3339)
	}
	return &dto.PublicProfileResponse{
		ID:           p.ID,
		Username:     p.Username,
		FullName:     p.FullName,
		AvatarURL:    p.AvatarURL,
		CoverURL:     p.CoverURL,
		Email:        p.Email,
		Phone:        p.Phone,
		FriendsCount: p.FriendsCount,
		Relation:     string(p.Relation),
		CreatedAt:    created,
	}
}

func (h *ProfileHandler) GetByUsername(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	username := chi.URLParam(r, "username")
	if username == "" {
		response.SendBadRequest(w, "username is required")
		return
	}

	p, err := h.svc.GetPublicProfile(r.Context(), viewerID, username)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, toPublicProfileDTO(p))
}
