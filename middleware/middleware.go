package middleware

import (
	"log"
	"net/http"

	"learn.ratelimiter/metrics"
	"learn.ratelimiter/types"
)

// RateLimitMiddleware provides rate limiting functionality.
type RateLimitMiddleware struct {
	limiter    types.Limiter
	metrics    *metrics.RateLimitMetrics
	limiterKey string
}

// NewRateLimitMiddleware creates a new RateLimitMiddleware.
// Added limiterKey parameter for logging.
func NewRateLimitMiddleware(limiter types.Limiter, metrics *metrics.RateLimitMetrics, limiterKey string) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter:    limiter,
		metrics:    metrics,
		limiterKey: limiterKey,
	}
}

// Handle wraps an http.HandlerFunc with rate limiting logic.
// identifierFunc is a function that extracts the identifier (e.g., IP address) from the request.
func (m *RateLimitMiddleware) Handle(next http.HandlerFunc, identifierFunc func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identifier := identifierFunc(r)
		if identifier == "" {
			// Log with RemoteAddr if identifier extraction fails
			log.Printf("Limiter '%s': Warning: Could not extract identifier for request from %s", m.limiterKey, r.RemoteAddr)
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("Limiter '%s': Request from %s denied due to missing identifier", m.limiterKey, r.RemoteAddr)
			m.metrics.RecordRequest(false)
			return
		}

		// Pass the request's context to the Allow method
		allowed, err := m.limiter.Allow(r.Context(), identifier)
		if err != nil {
			// Include limiter key and identifier in error log
			log.Printf("Limiter '%s': Error checking rate limit for identifier '%s': %v", m.limiterKey, identifier, err)
			w.WriteHeader(http.StatusInternalServerError)
			// Include limiter key and identifier in denial log
			log.Printf("Limiter '%s': Request for identifier '%s' denied due to limiter error", m.limiterKey, identifier)
			m.metrics.RecordRequest(false)
			return
		}

		m.metrics.RecordRequest(allowed)

		if allowed {
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusTooManyRequests)
			// Include limiter key, identifier, and path in denial log
			log.Printf("Limiter '%s': Request for identifier '%s' rate limited accessing %s", m.limiterKey, identifier, r.URL.Path)
		}
	}
}
