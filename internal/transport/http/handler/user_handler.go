package handler

import (
	"net/http"

	"github.com/unowned-22/api/internal/pagination"

	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// UserHandler handles user-scoped HTTP routes.
type UserHandler struct {
	userService user.UserService
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(userService user.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// Me returns the profile of the currently authenticated user.
func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	u, err := h.userService.GetProfile(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Role:      u.RoleName,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// List returns paginated users. Requires caller to have been authorized by middleware.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	q := pagination.ParseQuery(r)

	users, total, err := h.userService.ListUsers(r.Context(), q.Page, q.Limit)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	var out []dto.UserResponse
	for _, u := range users {
		out = append(out, dto.UserResponse{
			ID:        u.ID,
			Email:     u.Email,
			Role:      u.RoleName,
			CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	resp := pagination.BuildResponse(out, q.Page, q.Limit, total)
	response.SendSuccess(w, http.StatusOK, resp)
}
