package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/unowned-22/api/internal/transport/http/response"
	"golang.org/x/time/rate"
)

var (
	limiters = make(map[string]*rate.Limiter)
	mu       sync.Mutex
	once     sync.Once
)

// RateLimit returns a per-IP middleware using a token bucket limiter.
// limit is requests per second, burst is the maximum burst size.
func RateLimit(limit rate.Limit, burst int) func(http.Handler) http.Handler {
	// Start cleanup goroutine once
	once.Do(func() {
		go cleanupLimiters()
	})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			mu.Lock()
			limiter, exists := limiters[ip]
			if !exists {
				limiter = rate.NewLimiter(limit, burst)
				limiters[ip] = limiter
			}
			mu.Unlock()

			if !limiter.Allow() {
				response.SendTooManyRequests(w, "too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the client IP from X-Forwarded-For header (set by Caddy)
// or falls back to RemoteAddr.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (proxy/Caddy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// cleanupLimiters removes stale limiter entries older than 5 minutes.
func cleanupLimiters() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	lastSeen := make(map[string]time.Time)

	for range ticker.C {
		mu.Lock()

		now := time.Now()
		for ip := range limiters {
			if _, seen := lastSeen[ip]; !seen {
				lastSeen[ip] = now
			}
		}

		// Remove entries not seen in the last 5 minutes
		for ip, lastTime := range lastSeen {
			if now.Sub(lastTime) > 5*time.Minute {
				delete(limiters, ip)
				delete(lastSeen, ip)
			}
		}

		mu.Unlock()
	}
}
