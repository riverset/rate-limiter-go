package middleware

import (
	"log"
	"net/http"

	"learn.ratelimiter/metrics"
	"learn.ratelimiter/types"
)

// RateLimitMiddleware provides rate limiting functionality.
type RateLimitMiddleware struct {
	limiter types.Limiter
	metrics *metrics.RateLimitMetrics
}

// NewRateLimitMiddleware creates a new RateLimitMiddleware.
func NewRateLimitMiddleware(limiter types.Limiter, metrics *metrics.RateLimitMetrics) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter: limiter,
		metrics: metrics,
	}
}

// Handle wraps an http.HandlerFunc with rate limiting logic.
// identifierFunc is a function that extracts the identifier (e.g., IP address) from the request.
func (m *RateLimitMiddleware) Handle(next http.HandlerFunc, identifierFunc func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identifier := identifierFunc(r)
		if identifier == "" {
			log.Printf("Warning: Could not extract identifier for request from %s", r.RemoteAddr)
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("Request denied due to missing identifier for %s", r.RemoteAddr)
			m.metrics.RecordRequest(false)
			return
		}

		// Pass the request's context to the Allow method
		allowed, err := m.limiter.Allow(r.Context(), identifier)
		if err != nil {
			log.Printf("Error checking rate limit for %s: %v", identifier, err)
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("Request denied due to limiter error for %s", identifier)
			m.metrics.RecordRequest(false)
			return
		}

		m.metrics.RecordRequest(allowed)

		if allowed {
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusTooManyRequests)
			log.Printf("Request rate limited for %s accessing %s", identifier, r.URL.Path)
		}
	}
}
