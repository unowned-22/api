package dto

type CommentAuthorResponse struct {
	ID        int64  `json:"id"`
	FullName  string `json:"full_name"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

type CommentResponse struct {
	ID           int64                  `json:"id"`
	PhotoID      int64                  `json:"photo_id"`
	ParentID     *int64                 `json:"parent_id,omitempty"`
	Body         string                 `json:"body"`
	IsDeleted    bool                   `json:"is_deleted"`
	LikesCount   int                    `json:"likes_count"`
	RepliesCount int                    `json:"replies_count"`
	IsLiked      bool                   `json:"is_liked"`
	Author       *CommentAuthorResponse `json:"author,omitempty"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
}

type AddCommentRequest struct {
	ParentID *int64 `json:"parent_id,omitempty"`
	Body     string `json:"body"`
}

type EditCommentRequest struct {
	Body string `json:"body"`
}

type PaginatedCommentsResponse struct {
	Items  []*CommentResponse `json:"items"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}
