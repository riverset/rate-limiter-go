package metrics

import "sync/atomic"

type RateLimitMetrics struct {
	TotalRequests    int32
	RejectedRequests int32
	AllowedRequests  int32
}

func NewRateLimitMetrics() *RateLimitMetrics {
	return &RateLimitMetrics{}
}

func (r *RateLimitMetrics) RecordRequest(allowed bool) {
	atomic.AddInt32(&r.TotalRequests, 1)
	if allowed {
		atomic.AddInt32(&r.AllowedRequests, 1)
	} else {
		atomic.AddInt32(&r.RejectedRequests, 1)
	}
}
