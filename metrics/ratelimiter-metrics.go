// Package metrics contains code related to metrics and monitoring for the rate limiter.
package metrics

import "sync/atomic"

// RateLimitMetrics keeps track of rate limiting statistics.
type RateLimitMetrics struct {
	// TotalRequests is the total number of requests processed by the rate limiter.
	TotalRequests    int32
	// RejectedRequests is the number of requests that were rejected due to rate limiting.
	RejectedRequests int32
	// AllowedRequests is the number of requests that were allowed by the rate limiter.
	AllowedRequests  int32
}

// NewRateLimitMetrics creates a new instance of RateLimitMetrics.
func NewRateLimitMetrics() *RateLimitMetrics {
	return &RateLimitMetrics{}
}

// RecordRequest updates the metrics based on whether the request was allowed or rejected.
// It takes a boolean indicating if the request was allowed.
func (r *RateLimitMetrics) RecordRequest(allowed bool) {
	atomic.AddInt32(&r.TotalRequests, 1)
	if allowed {
		atomic.AddInt32(&r.AllowedRequests, 1)
	} else {
		atomic.AddInt32(&r.RejectedRequests, 1)
	}
}
