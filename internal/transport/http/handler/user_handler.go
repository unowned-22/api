package handler

import (
	"net/http"

	"github.com/unowned-22/api/internal/contextx"
	domain "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
)

type UserHandler struct {
	userService domain.UserService
}

// NewUserHandler creates a new instance of UserHandler
func NewUserHandler(userService domain.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// Me retrieves profile details of the currently logged-in user
func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	user, err := h.userService.Profile(r.Context(), userID)
	if err != nil {
		response.SendError(w, err)
		return
	}

	resp := dto.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Role:      user.RoleName,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	response.SendSuccess(w, http.StatusOK, resp)
}
