package pagination

import (
	"math"
	"net/http"
	"strconv"
)

const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 200
)

type Query struct {
	Page  int
	Limit int
}

func (q Query) Offset() int {
	if q.Page < 1 || q.Limit < 1 {
		return 0
	}
	return (q.Page - 1) * q.Limit
}

type Response[T any] struct {
	Data       []T   `json:"data"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

// ParseQuery extracts pagination params from the request's query string.
// Applies defaults and bounds.
func ParseQuery(r *http.Request) Query {
	qp := Query{Page: DefaultPage, Limit: DefaultLimit}
	if r == nil {
		return qp
	}
	qs := r.URL.Query()
	if p := qs.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			qp.Page = v
		}
	}
	if l := qs.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			if v > MaxLimit {
				v = MaxLimit
			}
			qp.Limit = v
		}
	}
	return qp
}

func BuildResponse[T any](items []T, page, limit int, total int64) Response[T] {
	totalPages := 0
	if limit > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(limit)))
	}
	return Response[T]{
		Data:       items,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}
}
