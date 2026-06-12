package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

// RequestID generates a unique Request ID for tracing, setting it in headers and context
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = generateRandomID(16)
		}

		w.Header().Set("X-Request-Id", reqID)
		ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from the context
func GetRequestID(ctx context.Context) string {
	if val, ok := ctx.Value(RequestIDKey).(string); ok {
		return val
	}
	return ""
}

func generateRandomID(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "fallback-request-id"
	}
	return hex.EncodeToString(bytes)
}
