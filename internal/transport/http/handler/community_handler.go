package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/community"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// CommunityHandler exposes community-related HTTP endpoints.
type CommunityHandler struct {
	svc community.Service
}

func NewCommunityHandler(svc community.Service) *CommunityHandler {
	return &CommunityHandler{svc: svc}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (h *CommunityHandler) toCommunityResponse(c *community.Community) *dto.CommunityResponse {
	return &dto.CommunityResponse{
		ID:               c.ID,
		OwnerID:          c.OwnerID,
		Type:             string(c.Type),
		Visibility:       string(c.Visibility),
		Name:             c.Name,
		Slug:             c.Slug,
		Description:      c.Description,
		AvatarURL:        c.AvatarKey,
		BannerURL:        c.BannerKey,
		MembersCount:     c.MembersCount,
		PostsCount:       c.PostsCount,
		SubscribersCount: c.SubscribersCount,
		VideosCount:      c.VideosCount,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
	}
}

func (h *CommunityHandler) toMemberResponse(m *community.Member) *dto.CommunityMemberResponse {
	return &dto.CommunityMemberResponse{
		CommunityID: m.CommunityID,
		UserID:      m.UserID,
		Role:        string(m.Role),
		JoinedAt:    m.JoinedAt,
	}
}

func parseCommunityID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id, err == nil
}

// ── POST /api/v1/communities ─────────────────────────────────────────────────

// Create handles community creation.
// POST /api/v1/communities
func (h *CommunityHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	var req dto.CreateCommunityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	c, err := h.svc.Create(r.Context(), userID, community.CreateRequest{
		Type:        community.Type(req.Type),
		Visibility:  community.Visibility(req.Visibility),
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
	})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusCreated, h.toCommunityResponse(c))
}

// ── GET /api/v1/communities/{id} ─────────────────────────────────────────────

// GetByID returns the public profile of a community.
// GET /api/v1/communities/{id}
func (h *CommunityHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseCommunityID(r)
	if !ok {
		response.SendBadRequest(w, "invalid community id")
		return
	}
	c, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, h.toCommunityResponse(c))
}

// ── PATCH /api/v1/communities/{id} ───────────────────────────────────────────

// Update patches mutable community fields (owner or admin).
// PATCH /api/v1/communities/{id}
func (h *CommunityHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	id, ok := parseCommunityID(r)
	if !ok {
		response.SendBadRequest(w, "invalid community id")
		return
	}

	var req dto.UpdateCommunityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	c, err := h.svc.Update(r.Context(), id, userID, community.UpdateRequest{
		Name:        req.Name,
		Description: req.Description,
		AvatarKey:   req.AvatarKey,
		BannerKey:   req.BannerKey,
	})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, h.toCommunityResponse(c))
}

// ── DELETE /api/v1/communities/{id} ──────────────────────────────────────────

// Delete soft-deletes a community (owner only).
// DELETE /api/v1/communities/{id}
func (h *CommunityHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	id, ok := parseCommunityID(r)
	if !ok {
		response.SendBadRequest(w, "invalid community id")
		return
	}
	if err := h.svc.SoftDelete(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendNoContent(w)
}

// ── PATCH /api/v1/communities/{id}/type ──────────────────────────────────────

// ChangeType changes the community type (owner only).
// PATCH /api/v1/communities/{id}/type
func (h *CommunityHandler) ChangeType(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	id, ok := parseCommunityID(r)
	if !ok {
		response.SendBadRequest(w, "invalid community id")
		return
	}

	var req dto.ChangeCommunityTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if req.Type == "" {
		response.SendBadRequest(w, "type is required")
		return
	}

	c, err := h.svc.ChangeType(r.Context(), id, userID, community.Type(req.Type))
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, h.toCommunityResponse(c))
}

// ── POST /api/v1/communities/{id}/join ───────────────────────────────────────

// Join lets the current user join (or subscribe to) a community.
// POST /api/v1/communities/{id}/join
func (h *CommunityHandler) Join(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	id, ok := parseCommunityID(r)
	if !ok {
		response.SendBadRequest(w, "invalid community id")
		return
	}
	if err := h.svc.Join(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "joined"})
}

// ── POST /api/v1/communities/{id}/leave ──────────────────────────────────────

// Leave removes the current user from a community.
// POST /api/v1/communities/{id}/leave
func (h *CommunityHandler) Leave(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	id, ok := parseCommunityID(r)
	if !ok {
		response.SendBadRequest(w, "invalid community id")
		return
	}
	if err := h.svc.Leave(r.Context(), id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "left"})
}

// ── GET /api/v1/communities/{id}/members ─────────────────────────────────────

// ListMembers returns paginated community members.
// GET /api/v1/communities/{id}/members
func (h *CommunityHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := parseCommunityID(r)
	if !ok {
		response.SendBadRequest(w, "invalid community id")
		return
	}
	limit, offset := getPaginationQueries(r)

	var roleFilter *community.MemberRole
	if rv := r.URL.Query().Get("role"); rv != "" {
		rf := community.MemberRole(rv)
		roleFilter = &rf
	}

	members, err := h.svc.ListMembers(r.Context(), id, roleFilter, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]*dto.CommunityMemberResponse, 0, len(members))
	for _, m := range members {
		out = append(out, h.toMemberResponse(m))
	}
	response.SendSuccess(w, http.StatusOK, dto.CommunityMemberListResponse{Members: out})
}

// ── POST /api/v1/communities/{id}/members/{userID}/role ──────────────────────

// SetMemberRole changes a member's role (owner only).
// POST /api/v1/communities/{id}/members/{userID}/role
func (h *CommunityHandler) SetMemberRole(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	communityID, ok := parseCommunityID(r)
	if !ok {
		response.SendBadRequest(w, "invalid community id")
		return
	}
	targetUserID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid user id")
		return
	}

	var req dto.SetMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if req.Role == "" {
		response.SendBadRequest(w, "role is required")
		return
	}

	if err := h.svc.SetMemberRole(r.Context(), communityID, requesterID, targetUserID, community.MemberRole(req.Role)); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "role updated"})
}

// ── GET /api/v1/communities/me/manageable ────────────────────────────────────

// ListManageable returns communities where the caller is owner or admin.
// GET /api/v1/communities/me/manageable
func (h *CommunityHandler) ListManageable(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	communities, err := h.svc.ListManageable(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]*dto.CommunityResponse, 0, len(communities))
	for _, c := range communities {
		out = append(out, h.toCommunityResponse(c))
	}
	response.SendSuccess(w, http.StatusOK, dto.CommunityListResponse{Communities: out})
}

// ── GET /api/v1/communities/search ───────────────────────────────────────────

// Search performs a fuzzy community search.
// GET /api/v1/communities/search?q=&type=
func (h *CommunityHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, offset := getPaginationQueries(r)

	var typeFilter *community.Type
	if tv := r.URL.Query().Get("type"); tv != "" {
		t := community.Type(tv)
		typeFilter = &t
	}

	communities, err := h.svc.Search(r.Context(), q, typeFilter, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]*dto.CommunityResponse, 0, len(communities))
	for _, c := range communities {
		out = append(out, h.toCommunityResponse(c))
	}
	response.SendSuccess(w, http.StatusOK, dto.CommunityListResponse{Communities: out})
}
