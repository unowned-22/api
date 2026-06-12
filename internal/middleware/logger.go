package middleware

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/unowned-22/api/internal/logger"
)

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
	rw.wroteHeader = true
}

func (rw *responseWriter) Write(buf []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(buf)
}

// Logger logs the details of each completed HTTP request in JSON format
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := newResponseWriter(w)

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		if logger.Log != nil {
			reqID := GetRequestID(r.Context())

			fields := logrus.Fields{
				"method":      r.Method,
				"path":        r.URL.Path,
				"status":      rw.status,
				"duration_ms": duration.Milliseconds(),
			}

			if reqID != "" {
				fields["request_id"] = reqID
			}

			// Info level as requested
			logger.Log.WithFields(fields).Info("request processed")
		}
	})
}
