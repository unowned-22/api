package handler

import (
	"net/http"

	domain "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/middleware"
	"github.com/unowned-22/api/internal/transport/http/response"
)

type UserHandler struct {
	userService domain.UserService
}

// NewUserHandler creates a new instance of UserHandler
func NewUserHandler(userService domain.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

type UserResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

// Me retrieves profile details of the currently logged-in user
func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	user, err := h.userService.Profile(r.Context(), userID)
	if err != nil {
		response.SendError(w, err)
		return
	}

	resp := UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Role:      user.RoleName,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	response.SendSuccess(w, http.StatusOK, resp)
}
