package middleware

import (
	"net/http"

	"github.com/rs/zerolog/log" // Import zerolog's global logger

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
			log.Warn().Str("limiter_key", m.limiterKey).Str("remote_addr", r.RemoteAddr).Msg("Middleware: Could not extract identifier for request")
			w.WriteHeader(http.StatusInternalServerError)
			log.Error().Str("limiter_key", m.limiterKey).Str("remote_addr", r.RemoteAddr).Msg("Middleware: Request denied due to missing identifier")
			m.metrics.RecordRequest(false)
			return
		}

		// Pass the request's context to the Allow method
		allowed, err := m.limiter.Allow(r.Context(), identifier)
		if err != nil {
			// Include limiter key and identifier in error log
			log.Error().Err(err).Str("limiter_key", m.limiterKey).Str("identifier", identifier).Msg("Middleware: Error checking rate limit")
			w.WriteHeader(http.StatusInternalServerError)
			// Include limiter key and identifier in denial log
			log.Error().Str("limiter_key", m.limiterKey).Str("identifier", identifier).Msg("Middleware: Request denied due to limiter error")
			m.metrics.RecordRequest(false)
			return
		}

		m.metrics.RecordRequest(allowed)

		if allowed {
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusTooManyRequests)
			// Include limiter key, identifier, and path in denial log
			log.Info().Str("limiter_key", m.limiterKey).Str("identifier", identifier).Str("path", r.URL.Path).Msg("Middleware: Request rate limited")
		}
	}
}
