package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chung-ng/system-design-lab/classical/01-rate-limiter/internal/limiter"
)

// KeyExtractor extracts a unique rate-limiting identifier from an HTTP request.
type KeyExtractor func(r *http.Request) string

// DefaultKeyExtractor checks for 'X-API-Key' header, falling back to Client IP.
func DefaultKeyExtractor(r *http.Request) string {
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "" {
		return "apikey:" + apiKey
	}

	// Fallback to IP address
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		parts := strings.Split(ip, ",")
		return "ip:" + strings.TrimSpace(parts[0])
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return "ip:" + host
	}
	return "ip:" + r.RemoteAddr
}

// ErrorResponse defines the standard payload returned upon HTTP 429.
type ErrorResponse struct {
	Error             string  `json:"error"`
	Message           string  `json:"message"`
	RetryAfterSeconds float64 `json:"retry_after_seconds"`
}

// RateLimitMiddleware returns an HTTP middleware wrapping endpoints with rate-limiting protection.
func RateLimitMiddleware(l limiter.RateLimiter, extractor KeyExtractor) func(http.Handler) http.Handler {
	if extractor == nil {
		extractor = DefaultKeyExtractor
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := extractor(r)

			// Bound rate limiter lookup with a strict 50ms timeout to prevent latency spikes
			ctx, cancel := context.WithTimeout(r.Context(), 50*time.Millisecond)
			defer cancel()

			res, err := l.Allow(ctx, key)
			if err != nil {
				http.Error(w, "Internal Server Error: Rate limiter unavailable", http.StatusInternalServerError)
				return
			}

			// Attach standard RFC draft rate limit headers
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(res.Remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(int64(res.ResetAfter.Seconds()), 10))

			if !res.Allowed {
				retryAfterSec := res.RetryAfter.Seconds()
				if retryAfterSec < 1.0 {
					retryAfterSec = 1.0
				}

				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", retryAfterSec))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)

				_ = json.NewEncoder(w).Encode(ErrorResponse{
					Error:             "Too Many Requests",
					Message:           "Rate limit exceeded. Please back off before retrying.",
					RetryAfterSeconds: retryAfterSec,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
