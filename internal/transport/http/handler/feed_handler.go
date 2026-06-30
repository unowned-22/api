package handler

import (
	"net/http"

	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/feed"
	"github.com/unowned-22/api/internal/service"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// FeedHandler exposes GET /api/v1/feed.
type FeedHandler struct {
	svc *service.FeedService
}

func NewFeedHandler(svc *service.FeedService) *FeedHandler {
	return &FeedHandler{svc: svc}
}

func toFeedItemResponse(it *feed.Item) *dto.FeedItemResponse {
	resp := &dto.FeedItemResponse{
		SourceType:    string(it.SourceType),
		ID:            it.ID,
		OwnerID:       it.OwnerID,
		CommunityID:   it.CommunityID,
		Text:          it.Text,
		LikesCount:    it.LikesCount,
		CommentsCount: it.CommentsCount,
		CreatedAt:     it.CreatedAt,
	}
	// Media is stored as raw JSONB; best-effort decode, fall back to empty list.
	if len(it.Media) > 0 {
		var media []dto.MediaItemResponse
		if err := jsonUnmarshalSafe(it.Media, &media); err == nil {
			resp.Media = media
		}
	}
	if resp.Media == nil {
		resp.Media = []dto.MediaItemResponse{}
	}
	return resp
}

// ListHomeFeed  GET /api/v1/feed?type=&limit=&offset=
func (h *FeedHandler) ListHomeFeed(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextx.UserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	limit, offset := getPaginationQueries(r)

	var typeFilter *string
	if tv := r.URL.Query().Get("type"); tv != "" {
		typeFilter = &tv
	}

	items, err := h.svc.ListHomeFeed(r.Context(), userID, typeFilter, limit, offset)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]*dto.FeedItemResponse, 0, len(items))
	for _, it := range items {
		out = append(out, toFeedItemResponse(it))
	}
	response.SendSuccess(w, http.StatusOK, dto.FeedResponse{Items: out})
}
