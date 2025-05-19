package middleware

import (
	"log"
	"net/http"

	ratelimiter "learn.ratelimiter/api"
	"learn.ratelimiter/metrics"
)

// RateLimitMiddleware provides rate limiting functionality.
type RateLimitMiddleware struct {
	limiter ratelimiter.Limiter
	metrics *metrics.RateLimitMetrics // Use the provided metrics type
}

// NewRateLimitMiddleware creates a new RateLimitMiddleware.
func NewRateLimitMiddleware(limiter ratelimiter.Limiter, metrics *metrics.RateLimitMetrics) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter: limiter,
		metrics: metrics,
	}
}

// Handle wraps an http.HandlerFunc with rate limiting logic.
// identifierFunc is a function that extracts the identifier (e.g., IP address) from the request.
func (m *RateLimitMiddleware) Handle(next http.HandlerFunc, identifierFunc func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Record the request regardless of outcome
		// m.metrics.TotalRequests++ // Removed direct access, use RecordRequest instead

		identifier := identifierFunc(r)
		if identifier == "" {
			// Handle cases where identifier cannot be extracted (e.g., deny or allow by default)
			log.Printf("Warning: Could not extract identifier for request from %s", r.RemoteAddr)
			// Decide on default behavior, e.g., deny to be safe
			w.WriteHeader(http.StatusInternalServerError) // Or http.StatusForbidden
			log.Printf("Request denied due to missing identifier for %s", r.RemoteAddr)
			m.metrics.RecordRequest(false) // Record as rejected
			return
		}

		allowed, err := m.limiter.Allow(identifier)
		if err != nil {
			// Handle errors from the limiter (e.g., backend connection issues)
			log.Printf("Error checking rate limit for %s: %v", identifier, err)
			// Decide on default behavior when limiter fails, e.g., deny to be safe
			w.WriteHeader(http.StatusInternalServerError) // Or http.StatusTooManyRequests, depending on policy
			log.Printf("Request denied due to limiter error for %s", identifier)
			m.metrics.RecordRequest(false) // Record as rejected
			return
		}

		m.metrics.RecordRequest(allowed) // Record the request outcome

		if allowed {
			// m.metrics.AllowedRequests++ // Removed direct access
			next.ServeHTTP(w, r) // Proceed to the next handler
		} else {
			// m.metrics.RejectedRequests++ // Removed direct access
			w.WriteHeader(http.StatusTooManyRequests) // 429 Too Many Requests
			log.Printf("Request rate limited for %s", identifier)
		}
	}
}
