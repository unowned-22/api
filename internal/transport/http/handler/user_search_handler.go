package handler

import (
	"net/http"
	"strconv"

	"github.com/unowned-22/api/internal/service"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// UserSearchHandler exposes read-only user search (mention pickers,
// "find a person" UI, etc). No privacy filtering is applied yet — every
// confirmed user is searchable by every authenticated user.
type UserSearchHandler struct {
	searchService *service.UserSearchService
}

func NewUserSearchHandler(searchService *service.UserSearchService) *UserSearchHandler {
	return &UserSearchHandler{searchService: searchService}
}

// Search handles GET /api/v1/users/search?q=&limit=
func (h *UserSearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}

	docs, err := h.searchService.Search(r.Context(), q, limit)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	items := make([]dto.UserSearchItem, 0, len(docs))
	for _, d := range docs {
		items = append(items, dto.UserSearchItem{
			ID:        d.ID,
			Username:  d.Username,
			FullName:  d.FullName,
			AvatarURL: d.AvatarURL,
		})
	}

	response.SendSuccess(w, http.StatusOK, dto.UserSearchResponse{Items: items})
}
