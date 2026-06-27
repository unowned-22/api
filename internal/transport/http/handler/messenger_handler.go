package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/contextx"
	"github.com/unowned-22/api/internal/domain/messenger"
	domainstorage "github.com/unowned-22/api/internal/domain/storage"
	"github.com/unowned-22/api/internal/logger"
	"github.com/unowned-22/api/internal/pagination"
	"github.com/unowned-22/api/internal/transport/http/dto"
	"github.com/unowned-22/api/internal/transport/http/response"
	"github.com/unowned-22/api/internal/validator"
)

type MessengerHandler struct {
	svc     messenger.Service
	storage domainstorage.Storage
	cfg     config.Config
}

func NewMessengerHandler(svc messenger.Service, storage domainstorage.Storage, cfg config.Config) *MessengerHandler {
	return &MessengerHandler{svc: svc, storage: storage, cfg: cfg}
}

func (h *MessengerHandler) currentUserID(r *http.Request) (int64, bool) {
	return contextx.UserID(r.Context())
}

func convDTO(c *messenger.Conversation) dto.ConversationResponse {
	return dto.ConversationResponse{
		ID:              c.ID,
		Type:            string(c.Type),
		Title:           c.Title,
		Description:     c.Description,
		AvatarURL:       c.AvatarURL,
		OwnerID:         c.OwnerID,
		CreatedBy:       c.CreatedBy,
		LastMessageID:   c.LastMessageID,
		LastMessageAt:   c.LastMessageAt,
		MembersCount:    c.MembersCount,
		IsArchived:      c.IsArchived,
		InviteLink:      c.InviteLink,
		DisappearAfterS: c.DisappearAfterS,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

func msgDTO(m *messenger.Message) dto.MessengerMessageResponse {
	out := dto.MessengerMessageResponse{
		ID:              m.ID,
		ConversationID:  m.ConversationID,
		SenderID:        m.SenderID,
		Type:            string(m.Type),
		Body:            m.Body,
		ReplyToID:       m.ReplyToID,
		ForwardedFromID: m.ForwardedFromID,
		IsDeleted:       m.IsDeleted,
		IsEdited:        m.IsEdited,
		EditedAt:        m.EditedAt,
		Pinned:          m.Pinned,
		DisappearsAt:    m.DisappearsAt,
		ScheduledAt:     m.ScheduledAt,
		IsScheduled:     m.IsScheduled,
		DeliveryStatus:  string(m.DeliveryStatus),
		MentionUserIDs:  m.MentionUserIDs,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
		SenderName:      m.SenderName,
		SenderAvatar:    m.SenderAvatar,
	}
	for _, r := range m.Reactions {
		out.Reactions = append(out.Reactions, dto.ReactionSummaryResponse{
			Emoji:       r.Emoji,
			Count:       r.Count,
			ReactedByMe: r.ReactedByMe,
		})
	}
	for _, a := range m.Attachments {
		out.Attachments = append(out.Attachments, dto.AttachmentResponse{
			ID:        a.ID,
			Type:      a.Type,
			URL:       a.URL,
			MimeType:  a.MimeType,
			SizeBytes: a.SizeBytes,
			Filename:  a.Filename,
			DurationS: a.DurationS,
			Width:     a.Width,
			Height:    a.Height,
		})
	}
	if m.ReplyTo != nil {
		preview := dto.MessagePreviewResponse{
			ID:         m.ReplyTo.ID,
			SenderID:   m.ReplyTo.SenderID,
			SenderName: m.ReplyTo.SenderName,
			Body:       m.ReplyTo.Body,
		}
		out.ReplyTo = &preview
	}
	return out
}

func (h *MessengerHandler) GetOrCreateDirect(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	targetID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid user id")
		return
	}
	conv, err := h.svc.GetOrCreateDirect(r.Context(), userID, targetID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, convDTO(conv))
}

func (h *MessengerHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	var req dto.CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.SendError(w, r, err)
		return
	}
	conv, err := h.svc.CreateGroup(r.Context(), userID, req.Title, req.Description, req.MemberIDs)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusCreated, convDTO(conv))
}

func (h *MessengerHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	var req dto.CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.SendError(w, r, err)
		return
	}
	conv, err := h.svc.CreateChannel(r.Context(), userID, req.Title, req.Description)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusCreated, convDTO(conv))
}

func (h *MessengerHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	items, total, err := h.svc.ListConversations(r.Context(), userID, pagination.ParseQuery(r))
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.ConversationResponse, 0, len(items))
	for _, c := range items {
		out = append(out, convDTO(c))
	}
	response.SendSuccess(w, http.StatusOK, dto.ConversationsResponse{Items: out, Total: total})
}

func (h *MessengerHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	conv, err := h.svc.GetConversation(r.Context(), userID, id)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, convDTO(conv))
}

func (h *MessengerHandler) GetPrivacy(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	settings, err := h.svc.GetPrivacySettings(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.PrivacyResponse{UserID: settings.UserID, WhoCanMessage: string(settings.WhoCanMessage), UpdatedAt: settings.UpdatedAt.Format(time.RFC3339)})
}

func (h *MessengerHandler) UpdatePrivacy(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	var req dto.UpdatePrivacyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.SendError(w, r, err)
		return
	}
	err := h.svc.UpdatePrivacySettings(r.Context(), userID, &messenger.MessengerPrivacySettings{WhoCanMessage: messenger.WhoCanMessage(req.WhoCanMessage)})
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "privacy settings updated"})
}

func (h *MessengerHandler) BlockUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	blockedID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid user id")
		return
	}
	if err := h.svc.BlockUser(r.Context(), userID, blockedID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "user blocked"})
}

func (h *MessengerHandler) UnblockUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	blockedID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid user id")
		return
	}
	if err := h.svc.UnblockUser(r.Context(), userID, blockedID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "user unblocked"})
}

func (h *MessengerHandler) ListBlocked(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	items, err := h.svc.ListBlocked(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.BlockedUsersResponse{UserIDs: items})
}

func (h *MessengerHandler) AddMembers(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req struct {
		MemberIDs []int64 `json:"member_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if err := h.svc.AddMembers(r.Context(), userID, convID, req.MemberIDs); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "members added"})
}

func (h *MessengerHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	memberID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid user id")
		return
	}
	if err := h.svc.RemoveMember(r.Context(), userID, convID, memberID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "member removed"})
}

func (h *MessengerHandler) LeaveConversation(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.LeaveConversation(r.Context(), userID, convID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "left conversation"})
}

func (h *MessengerHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.Subscribe(r.Context(), userID, convID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "subscribed"})
}

func (h *MessengerHandler) ArchiveConversation(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.ArchiveConversation(r.Context(), userID, convID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "conversation archived"})
}

func (h *MessengerHandler) UnarchiveConversation(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.UnarchiveConversation(r.Context(), userID, convID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "conversation unarchived"})
}

func (h *MessengerHandler) GenerateInviteLink(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	link, err := h.svc.GenerateInviteLink(r.Context(), userID, convID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.InviteLinkResponse{Link: link})
}

func (h *MessengerHandler) RevokeInviteLink(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.RevokeInviteLink(r.Context(), userID, convID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "invite link revoked"})
}

func (h *MessengerHandler) JoinByInviteLink(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		response.SendBadRequest(w, "slug is required")
		return
	}
	conv, err := h.svc.JoinByInviteLink(r.Context(), userID, slug)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, convDTO(conv))
}

func (h *MessengerHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req dto.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}

	base := strings.TrimSuffix(h.cfg.StoragePublicEndpoint, "/")
	attachments := make([]messenger.Attachment, 0, len(req.AttachmentKeys))
	for _, a := range req.AttachmentKeys {
		info, err := h.storage.StatObject(r.Context(), h.cfg.MinIOBucket, a)
		if err != nil {
			logger.Log.WithError(err).Warnf("SendMessage: failed to stat object %s", a)
			continue
		}

		filename := info.Metadata["filename"]
		if filename == "" {
			filename = path.Base(a)
		}

		attachments = append(attachments, messenger.Attachment{
			Type:       attachmentTypeFromMime(info.ContentType),
			StorageKey: a,
			URL:        fmt.Sprintf("%s/%s/%s", base, h.cfg.MinIOBucket, a),
			MimeType:   info.ContentType,
			SizeBytes:  info.Size,
			Filename:   filename,
		})
	}

	msg, err := h.svc.SendMessage(r.Context(), userID, convID, &messenger.Message{
		Type:      messenger.MessageTypeText,
		Body:      req.Body,
		ReplyToID: req.ReplyToID,
	}, attachments)

	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusCreated, msgDTO(msg))
}

func attachmentTypeFromMime(mime string) string {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	default:
		return "document"
	}
}

func (h *MessengerHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	items, total, err := h.svc.ListMessages(r.Context(), userID, convID, pagination.ParseQuery(r))
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.MessengerMessageResponse, 0, len(items))
	for _, m := range items {
		out = append(out, msgDTO(m))
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageListResponse{Items: out, Total: total})
}

func (h *MessengerHandler) SearchMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	query := r.URL.Query().Get("q")
	items, total, err := h.svc.SearchMessages(r.Context(), userID, convID, query, pagination.ParseQuery(r))
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.MessengerMessageResponse, 0, len(items))
	for _, m := range items {
		out = append(out, msgDTO(m))
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageListResponse{Items: out, Total: total})
}

func (h *MessengerHandler) ListPinned(w http.ResponseWriter, r *http.Request) {
	response.SendNotImplemented(w, "listing pinned messages is not implemented yet")
}

func (h *MessengerHandler) EditMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	msgID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	msg, err := h.svc.EditMessage(r.Context(), userID, msgID, req.Body)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, msgDTO(msg))
}

func (h *MessengerHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	msgID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.DeleteMessage(r.Context(), userID, msgID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "message deleted"})
}

func (h *MessengerHandler) PinMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	msgID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.PinMessage(r.Context(), userID, 0, msgID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "message pinned"})
}

func (h *MessengerHandler) UnpinMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	msgID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.UnpinMessage(r.Context(), userID, 0, msgID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "message unpinned"})
}

func (h *MessengerHandler) ForwardMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	msgID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req dto.ForwardMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if err := h.svc.ForwardMessage(r.Context(), userID, msgID, req.TargetConversationIDs); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "message forwarded"})
}

func (h *MessengerHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req dto.MarkReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if err := h.svc.MarkRead(r.Context(), userID, convID, req.LastMessageID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "conversation marked as read"})
}

func (h *MessengerHandler) ScheduleMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req dto.ScheduleMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	msg, err := h.svc.ScheduleMessage(r.Context(), userID, convID, &messenger.Message{Type: messenger.MessageTypeText, Body: req.Body}, nil, req.SendAt)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusCreated, msgDTO(msg))
}

func (h *MessengerHandler) ListScheduled(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid conversation id")
		return
	}
	msgs, err := h.svc.ListScheduledMessages(r.Context(), userID, convID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	out := make([]dto.MessengerMessageResponse, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, msgDTO(m))
	}

	response.SendSuccess(w, http.StatusOK, out)
}

func (h *MessengerHandler) CancelScheduled(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	msgID, err := strconv.ParseInt(chi.URLParam(r, "msgID"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid message id")
		return
	}
	if err := h.svc.CancelScheduledMessage(r.Context(), userID, msgID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendNoContent(w)
}

func (h *MessengerHandler) Typing(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid conversation id")
		return
	}
	var req struct {
		IsTyping bool `json:"is_typing"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid body")
		return
	}
	if err := h.svc.SendTyping(r.Context(), userID, convID, req.IsTyping); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendNoContent(w)
}

func (h *MessengerHandler) SaveDraft(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req dto.SaveDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if err := h.svc.SaveDraft(r.Context(), userID, convID, req.Body, req.ReplyToID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "draft saved"})
}

func (h *MessengerHandler) GetDraft(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	draft, err := h.svc.GetDraft(r.Context(), userID, convID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, draft)
}

func (h *MessengerHandler) DeleteDraft(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	convID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	if err := h.svc.DeleteDraft(r.Context(), userID, convID); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "draft deleted"})
}

func (h *MessengerHandler) SetDisappearTimer(w http.ResponseWriter, r *http.Request) {
	response.SendNotImplemented(w, "disappear timer is not implemented yet")
}

func (h *MessengerHandler) ListMentions(w http.ResponseWriter, r *http.Request) {
	response.SendNotImplemented(w, "mention listing is not implemented yet")
}

func (h *MessengerHandler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 222*1024*1024)
	mr, err := r.MultipartReader()
	if err != nil {
		response.SendBadRequest(w, "invalid multipart request")
		return
	}

	var part *multipart.Part
	for p, pErr := mr.NextPart(); pErr == nil; p, pErr = mr.NextPart() {
		if p.FormName() == "file" {
			part = p
			break
		}
	}
	if part == nil {
		response.SendBadRequest(w, "file part is required")
		return
	}

	contentType := part.Header.Get("Content-Type")
	data, err := io.ReadAll(part)
	if err != nil {
		response.SendBadRequest(w, "failed to read file")
		return
	}

	storageKey, url, err := h.svc.UploadAttachment(r.Context(), userID, part.FileName(), contentType, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendSuccess(w, http.StatusOK, dto.UploadAttachmentResponse{StorageKey: storageKey, URL: url})
}

func (h *MessengerHandler) AddReaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	msgID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	var req struct {
		Emoji string `json:"emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.SendBadRequest(w, "invalid request body")
		return
	}
	if err := h.svc.AddReaction(r.Context(), userID, msgID, req.Emoji); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "reaction added"})
}

func (h *MessengerHandler) RemoveReaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	msgID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.SendBadRequest(w, "invalid id")
		return
	}
	emoji := chi.URLParam(r, "emoji")
	if err := h.svc.RemoveReaction(r.Context(), userID, msgID, emoji); err != nil {
		response.SendError(w, r, err)
		return
	}
	response.SendSuccess(w, http.StatusOK, dto.MessageResponse{Message: "reaction removed"})
}
