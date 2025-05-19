package metrics

import "sync/atomic"

// RateLimitMetrics keeps track of rate limiting statistics.
type RateLimitMetrics struct {
	TotalRequests    int32
	RejectedRequests int32
	AllowedRequests  int32
}

// NewRateLimitMetrics creates a new instance of RateLimitMetrics.
func NewRateLimitMetrics() *RateLimitMetrics {
	return &RateLimitMetrics{}
}

// RecordRequest updates the metrics based on whether the request was allowed or rejected.
func (r *RateLimitMetrics) RecordRequest(allowed bool) {
	atomic.AddInt32(&r.TotalRequests, 1)
	if allowed {
		atomic.AddInt32(&r.AllowedRequests, 1)
	} else {
		atomic.AddInt32(&r.RejectedRequests, 1)
	}
}
