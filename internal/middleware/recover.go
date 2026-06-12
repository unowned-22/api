package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/unowned-22/api/internal/logger"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// Recover intercepts panics during request execution, logs them, and returns a clean 500 response
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()

				if logger.Log != nil {
					reqID := GetRequestID(r.Context())
					logger.Log.WithFields(map[string]interface{}{
						"panic":      err,
						"stack":      string(stack),
						"request_id": reqID,
					}).Error("panic recovered")
				}

				response.SendError(w, fmt.Errorf("panic: %v", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}
