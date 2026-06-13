package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/unowned-22/api/internal/contextx"
)

// RequestID generates a unique Request ID for tracing, setting it in headers and context
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = generateRandomID(16)
		}

		w.Header().Set("X-Request-Id", reqID)
		ctx := contextx.SetRequestID(r.Context(), reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateRandomID(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "fallback-request-id"
	}
	return hex.EncodeToString(bytes)
}
