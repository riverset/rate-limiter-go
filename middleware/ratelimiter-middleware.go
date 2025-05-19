package middleware

// import (
// 	"fmt"
// 	"net/http"

// 	ratelimiter "learn.ratelimiter/api"
// 	"learn.ratelimiter/metrics"
// )

// type RateLimitMiddleware struct {
// 	rateLimiter ratelimiter.Limiter
// 	metrics     *metrics.RateLimitMetrics
// }

// func NewRateLimitMiddleware(limiter ratelimiter.Limiter, metrics *metrics.RateLimitMetrics) *RateLimitMiddleware {
// 	return &RateLimitMiddleware{
// 		rateLimiter: limiter,
// 		metrics:     metrics,
// 	}
// }

// func (m *RateLimitMiddleware) Handle(next http.HandlerFunc, getIdentifierFunc func(*http.Request) string) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		ip := getIdentifierFunc(r)
// 		allowed := m.rateLimiter.Allow(ip)

// 		m.metrics.RecordRequest(allowed)

// 		if !allowed {
// 			w.WriteHeader(http.StatusTooManyRequests)
// 			fmt.Fprintln(w, "Rate limit exceeded. Please try again later.")
// 			return
// 		}
// 		next(w, r)
// 	}
// }
