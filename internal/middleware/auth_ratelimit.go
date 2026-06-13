package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/unowned-22/api/internal/logger"
	"github.com/unowned-22/api/internal/transport/http/response"
)

// AuthRateLimiterConfig holds the configuration for auth endpoint rate limiting.
type AuthRateLimiterConfig struct {
	Limit  int           // Maximum attempts allowed
	Window time.Duration // Time window for counting attempts
}

// requestRecord tracks a single request with timestamp.
type requestRecord struct {
	timestamp  time.Time
	identifier string // IP or email
}

// AuthRateLimiter tracks requests per endpoint and identifier (IP or email).
type AuthRateLimiter struct {
	config        AuthRateLimiterConfig
	records       map[string][]requestRecord
	mu            sync.RWMutex
	cleanupTicker *time.Ticker
}

// NewAuthRateLimiter creates a new auth rate limiter with cleanup goroutine.
func NewAuthRateLimiter(config AuthRateLimiterConfig) *AuthRateLimiter {
	limiter := &AuthRateLimiter{
		config:        config,
		records:       make(map[string][]requestRecord),
		cleanupTicker: time.NewTicker(1 * time.Minute),
	}

	// Start cleanup goroutine
	go limiter.cleanupExpired()

	return limiter
}

// Allow checks if a request should be allowed based on the identifier and window.
// Returns (allowed, remaining_attempts).
func (l *AuthRateLimiter) Allow(identifier string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-l.config.Window)

	// Get or create records for this identifier
	records := l.records[identifier]

	// Remove expired records
	var validRecords []requestRecord
	for _, r := range records {
		if r.timestamp.After(windowStart) {
			validRecords = append(validRecords, r)
		}
	}

	count := len(validRecords)
	remaining := l.config.Limit - count

	if count >= l.config.Limit {
		// Limit exceeded
		l.records[identifier] = validRecords
		return false, 0
	}

	// Record this request
	validRecords = append(validRecords, requestRecord{
		timestamp:  now,
		identifier: identifier,
	})
	l.records[identifier] = validRecords

	return true, remaining - 1
}

// cleanupExpired removes stale entries older than the window.
func (l *AuthRateLimiter) cleanupExpired() {
	for range l.cleanupTicker.C {
		l.mu.Lock()

		now := time.Now()
		windowStart := now.Add(-l.config.Window)

		for identifier, records := range l.records {
			var validRecords []requestRecord
			for _, r := range records {
				if r.timestamp.After(windowStart) {
					validRecords = append(validRecords, r)
				}
			}

			if len(validRecords) == 0 {
				delete(l.records, identifier)
			} else {
				l.records[identifier] = validRecords
			}
		}

		l.mu.Unlock()
	}
}

// Stop stops the cleanup goroutine.
func (l *AuthRateLimiter) Stop() {
	if l.cleanupTicker != nil {
		l.cleanupTicker.Stop()
	}
}

// AuthRateLimitByIP returns middleware that rate limits based on client IP.
func AuthRateLimitByIP(endpoint string, limiter *AuthRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			allowed, remaining := limiter.Allow(ip)

			if !allowed {
				logger.Log.WithFields(map[string]interface{}{
					"endpoint": endpoint,
					"ip":       ip,
					"limit":    limiter.config.Limit,
					"window":   limiter.config.Window.String(),
				}).Warn("auth endpoint rate limit exceeded (IP)")

				response.SendTooManyRequests(w, fmt.Sprintf("rate limit exceeded for %s endpoint", endpoint))
				return
			}

			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			next.ServeHTTP(w, r)
		})
	}
}

// AuthRateLimitByEmail returns middleware that rate limits based on email from request body.
// This is for endpoints that have an email field in the JSON body (login, register, forgot-password).
func AuthRateLimitByEmail(endpoint string, limiter *AuthRateLimiter, emailExtractor func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email := emailExtractor(r)
			if email == "" {
				// If email cannot be extracted, fall back to IP-based limiting
				ip := getClientIP(r)
				allowed, remaining := limiter.Allow(ip)
				if !allowed {
					logger.Log.WithFields(map[string]interface{}{
						"endpoint": endpoint,
						"ip":       ip,
						"limit":    limiter.config.Limit,
						"window":   limiter.config.Window.String(),
					}).Warn("auth endpoint rate limit exceeded (IP fallback)")

					response.SendTooManyRequests(w, fmt.Sprintf("rate limit exceeded for %s endpoint", endpoint))
					return
				}
				w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
				next.ServeHTTP(w, r)
				return
			}

			// Limit by both IP and email
			ip := getClientIP(r)
			ipKey := fmt.Sprintf("%s:ip:%s", endpoint, ip)
			emailKey := fmt.Sprintf("%s:email:%s", endpoint, email)

			ipAllowed, ipRemaining := limiter.Allow(ipKey)
			emailAllowed, emailRemaining := limiter.Allow(emailKey)

			if !ipAllowed {
				logger.Log.WithFields(map[string]interface{}{
					"endpoint": endpoint,
					"ip":       ip,
					"limit":    limiter.config.Limit,
					"window":   limiter.config.Window.String(),
				}).Warn("auth endpoint rate limit exceeded (IP)")

				response.SendTooManyRequests(w, fmt.Sprintf("rate limit exceeded for %s endpoint", endpoint))
				return
			}

			if !emailAllowed {
				logger.Log.WithFields(map[string]interface{}{
					"endpoint": endpoint,
					"email":    email,
					"limit":    limiter.config.Limit,
					"window":   limiter.config.Window.String(),
				}).Warn("auth endpoint rate limit exceeded (email)")

				response.SendTooManyRequests(w, fmt.Sprintf("rate limit exceeded for %s endpoint", endpoint))
				return
			}

			// Use the lower remaining count for the header
			remaining := ipRemaining
			if emailRemaining < remaining {
				remaining = emailRemaining
			}

			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			next.ServeHTTP(w, r)
		})
	}
}
