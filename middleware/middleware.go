// Package middleware provides HTTP middleware for integrating the rate limiter.
package middleware

import (
	"net/http"

	"github.com/rs/zerolog/log" // Import zerolog's global logger

	"learn.ratelimiter/config"
	"learn.ratelimiter/metrics"
	"learn.ratelimiter/types"
)

// RateLimitMiddleware provides rate limiting functionality for HTTP handlers.
type RateLimitMiddleware struct {
	// limiter is the rate limiter instance to use.
	limiter types.Limiter
	// metrics is the metrics collector for rate limiting statistics.
	metrics *metrics.RateLimitMetrics
	// limiterKey is the key associated with this limiter configuration.
	limiterKey string
	// algorithm is the rate limiting algorithm used by this limiter.
	algorithm config.AlgorithmType
}

// NewRateLimitMiddleware creates a new RateLimitMiddleware.
// It takes a types.Limiter, a metrics.RateLimitMetrics collector, a unique key for the limiter, and the algorithm type.
func NewRateLimitMiddleware(limiter types.Limiter, metrics *metrics.RateLimitMetrics, limiterKey string, algorithm config.AlgorithmType) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter:    limiter,
		metrics:    metrics,
		limiterKey: limiterKey,
		algorithm:  algorithm,
	}
}

// Handle wraps an http.HandlerFunc with rate limiting logic.
// It takes the next http.HandlerFunc in the chain and a function to extract the identifier from the request.
// It returns a new http.HandlerFunc that applies rate limiting before calling the next handler.
func (m *RateLimitMiddleware) Handle(next http.HandlerFunc, identifierFunc func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identifier := identifierFunc(r)
		if identifier == "" {
			// Log with RemoteAddr if identifier extraction fails
			log.Warn().Str("limiter_key", m.limiterKey).Str("remote_addr", r.RemoteAddr).Msg("Middleware: Could not extract identifier for request")
			w.WriteHeader(http.StatusInternalServerError)
			log.Error().Str("limiter_key", m.limiterKey).Str("remote_addr", r.RemoteAddr).Msg("Middleware: Request denied due to missing identifier")
			m.metrics.RecordRequestWithLabels(false, m.limiterKey, string(m.algorithm))
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
			m.metrics.RecordRequestWithLabels(false, m.limiterKey, string(m.algorithm))
			return
		}

		m.metrics.RecordRequestWithLabels(allowed, m.limiterKey, string(m.algorithm))

		if allowed {
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusTooManyRequests)
			// Include limiter key, identifier, and path in denial log
			log.Info().Str("limiter_key", m.limiterKey).Str("identifier", identifier).Str("path", r.URL.Path).Msg("Middleware: Request rate limited")
		}
	}
}
